package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// ModelGenerator generates models
type ModelGenerator struct {
	modelsPackage string
	schemasDir    string
	objects       map[string]*Object
	models        map[string]string
}

// Property repredents field definition
type Property struct {
	propType   string
	propFormat string
	propObjRef string
}

// Object represents object structure
type Object struct {
	name       string
	modelName  string
	refID      string
	properties map[string]*Property

	buf bytes.Buffer
}

const (
	packageTemplate = `
        // THIS IS A GENERATED FILE. DO NOT EDIT
        // Package %[1]s
        package %[1]s
`

	structTemplate = `
        // %[1]s model structure
        type %[1]s struct {
            %[2]s
        }
`
	fieldTemplate = "\t%[1]s %[2]s `json:\"%[3]s\"`\n"
)

func (mg *ModelGenerator) processProp(propName string, propData map[string]interface{}) *Property {
	prop := new(Property)
	if propType, ok := propData["type"].(string); ok {
		prop.propType = propType
		switch propType {
		case "string":
			if format, ok := propData["format"].(string); ok {
				prop.propFormat = format
			}
		case "object":
			obj := mg.processObject(propData, propName)
			prop.propObjRef = obj.refID

		case "array":
			if itemsProps, ok := propData["items"].(map[string]interface{}); ok {
				obj := mg.processObject(itemsProps, propName+"Item")
				prop.propObjRef = obj.refID
			}
		}
	} else {
		log.Fatalln("No type found for prop ", propName)
	}

	return prop
}

func (mg *ModelGenerator) processObject(data map[string]interface{}, name ...string) *Object {
	obj := new(Object)
	obj.properties = make(map[string]*Property)

	if refID, ok := data["$ref"].(string); ok {
		log.Println("Found reference object", refID)
		obj.refID = refID
	} else {
		if id, ok := data["id"].(string); ok {
			obj.refID = id
			mg.objects[id] = obj
		}

		if len(name) > 0 {
			fileName := name[0]
			obj.modelName = fileName[0 : len(fileName)-len(filepath.Ext(fileName))]
			mg.objects[fileName] = obj
			mg.models[obj.modelName] = fileName
			if len(obj.refID) == 0 {
				obj.refID = fileName
			}
		}

		if properties, ok := data["properties"].(map[string]interface{}); ok {
			for propName, propData := range properties {
				obj.properties[propName] = mg.processProp(propName, propData.(map[string]interface{}))
			}
		}
	}
	return obj
}

func (mg *ModelGenerator) processFile(fileName string) (err error) {
	log.Println("Processing '" + fileName + "'...")
	if contents, err := ioutil.ReadFile(fileName); err == nil {
		var data map[string]interface{}
		if err = json.Unmarshal(contents, &data); err == nil {
			_, fileName := filepath.Split(fileName)
			mg.processObject(data, fileName)
		}
	}
	return err
}

func (mg *ModelGenerator) generateField(fieldsBuf *bytes.Buffer, propName string, propData *Property) {
	var (
		fieldName string
		fieldType string
	)

	fieldName = strings.Title(propName)

	switch propData.propType {
	case "string":
		switch propData.propFormat {
		case "int64", "uint64":
			fieldType = propData.propFormat
		// case "date":
		// case "date-time":
		// case "byte":
		default:
			fieldType = "string"
		}
		propName += ",string"
	case "array":
		refModel, found := mg.objects[propData.propObjRef]
		if !found {
			log.Fatalf("Referenced model %s is not found\n", propData.propObjRef)
		}
		fieldType = "[]" + refModel.modelName
	case "boolean":
		fieldType = "bool"

	}

	fmt.Fprintf(fieldsBuf, fieldTemplate, fieldName, fieldType, propName)

	log.Println("\t\t"+propName, "of type", propData.propType)
	if len(propData.propObjRef) > 0 {
		_, found := mg.objects[propData.propObjRef]
		log.Println("\t\t... referenced to", propData.propObjRef, "found:", found)
	}

}

func (mg *ModelGenerator) generateModel(obj *Object) {
	outputName := filepath.Join(mg.modelsPackage, obj.modelName+".go")
	log.Println(outputName)

	var packageName = "main"

	if mg.modelsPackage != "" {
		packageName = mg.modelsPackage
		os.Mkdir(mg.modelsPackage, 0755)
	}
	fmt.Fprintf(&obj.buf, packageTemplate, packageName)

	var fieldsBuf bytes.Buffer

	for propName, propData := range obj.properties {
		mg.generateField(&fieldsBuf, propName, propData)
	}

	fmt.Fprintf(&obj.buf, structTemplate, obj.modelName, fieldsBuf.String())
	var err error
	src, err := format.Source(obj.buf.Bytes())
	if err != nil {
		// Should never happen, but can arise when developing this code.
		// The user can compile the output to see the error.
		log.Printf("warning: internal error: invalid Go generated: %s", err)
		log.Printf("warning: compile the package to analyze the error")
	}

	err = ioutil.WriteFile(outputName, src, 0644)
	if err != nil {
		log.Fatalf("writing output: %s", err)
	}

}

// Generate processes directory with schemas and generates objects
func (mg *ModelGenerator) Generate() error {
	var err error
	if files, err := filepath.Glob(filepath.Join(mg.schemasDir, "*.json")); err == nil {
		for _, fileName := range files {
			if fileInfo, err := os.Stat(fileName); err == nil && !fileInfo.IsDir() {
				err = mg.processFile(fileName)
			}
		}
	}

	log.Println("============================================")
	log.Println("Collected models:")
	for model, file := range mg.models {
		log.Println("\t", model, "from file", file)
		if obj, ok := mg.objects[model]; ok {
			mg.generateModel(obj)
		} else {
			log.Println("No object found")
		}
	}
	return err
}

// NewModelGenerator creates model generator
func NewModelGenerator(schemasDir string, modelsPackage string) *ModelGenerator {
	generator := new(ModelGenerator)
	generator.schemasDir = schemasDir
	generator.modelsPackage = modelsPackage
	generator.objects = make(map[string]*Object)
	generator.models = make(map[string]string)
	return generator
}
