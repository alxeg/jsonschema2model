package main

import (
	"github.com/alxeg/jsonschema/generator"
	flag "github.com/ogier/pflag"
	"log"
)

var (
	schemasDir  string
	packageName string
)

func init() {
	flag.StringVar(&schemasDir, "schemas-dir", ".", "Directory containing json schemas")
	flag.StringVar(&packageName, "package-name", "", "Package for generated sources")
}

func main() {
	log.SetPrefix("jsonschema: ")

	flag.Parse()

	log.Println("Generating...")

	gen := generator.NewModelGenerator(schemasDir, packageName)
	gen.Generate()
}
