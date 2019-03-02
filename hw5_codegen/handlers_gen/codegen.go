package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"reflect"
	"strings"
	"text/template"
)

type tpl struct {
	FieldName string
}

var (
	intTpl = template.Must(template.New("intTpl").Parse(`
	// {{.FieldName}}
	var {{.FieldName}}Raw uint32
	binary.Read(r, binary.LittleEndian, &{{.FieldName}}Raw)
	in.{{.FieldName}} = int({{.FieldName}}Raw)
`))

	strTpl = template.Must(template.New("strTpl").Parse(`
	// {{.FieldName}}
	var {{.FieldName}}LenRaw uint32
	binary.Read(r, binary.LittleEndian, &{{.FieldName}}LenRaw)
	{{.FieldName}}Raw := make([]byte, {{.FieldName}}LenRaw)
	binary.Read(r, binary.LittleEndian, &{{.FieldName}}Raw)
	in.{{.FieldName}} = string({{.FieldName}}Raw)
`))
)

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, _ := os.Create(os.Args[2])

	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, `import "encoding/binary"`)
	fmt.Fprintln(out, `import "bytes"`)
	fmt.Fprintln(out) // empty line

	fmt.Printf("First run for pkg: %s in file %s", node.Name.Name, os.Args[1])
FUNC_LOOP:
	for _, f := range node.Decls {
		fun, ok := f.(*ast.FuncDecl)
		if !ok {
			fmt.Printf("SKIP %T is not *ast.FuncDecl\n", f)
			continue
		}
		fmt.Printf("ANALYZE func %#v \n", fun.Name.Name)
		if fun.Doc == nil {
			fmt.Printf("SKIP func %#v doesnt have comments\n", fun.Name.Name)
			continue
		}
		needCodegen := false
		jsonStruct := ""
		name := ""
		for _, comment := range fun.Doc.List {
			PREFIX := "// apigen:api"
			needCodegen = needCodegen || strings.HasPrefix(comment.Text, PREFIX)
			jsonStruct = jsonStruct + comment.Text[len(PREFIX):]
		}
		if !needCodegen {
			fmt.Printf("SKIP func %#v doesnt have api mark\n", fun.Name.Name)
			continue FUNC_LOOP
		}

		fmt.Printf("meta func %#v %s \n", fun.Name.Name, jsonStruct)
		if field := fun.Recv.List[0]; field != nil {
			if star, ok := field.Type.(*ast.StarExpr); ok {
				if ident, ok := star.X.(*ast.Ident); ok {
					name = ident.Name
					fmt.Printf("need to generate type %s\n", name)
				}
			}
		}
		for _, paramfield := range fun.Type.Params.List {
			fmt.Printf("field: %#v \n", paramfield.Type)
		}

	}
	for _, f := range node.Decls {
		g, ok := f.(*ast.GenDecl)
		if !ok {
			fmt.Printf("SKIP %T is not *ast.GenDecl\n", f)
			continue
		}
	SPECS_LOOP:
		for _, spec := range g.Specs {
			currType, ok := spec.(*ast.TypeSpec)
			if !ok {
				fmt.Printf("SKIP %T is not ast.TypeSpec\n", spec)
				continue
			}

			currStruct, ok := currType.Type.(*ast.StructType)
			if !ok {
				fmt.Printf("SKIP %T is not ast.StructType\n", currStruct)
				continue
			}

			needCodegen := true
			//jsonStruct := ""
			//for _, comment := range g.Doc.List {
			//
			//}
			if !needCodegen {
				fmt.Printf("SKIP struct %#v doesnt needed by func pass \n", currType.Name.Name)
				continue SPECS_LOOP
			}

			fmt.Printf("process struct %s\n", currType.Name.Name)

			fmt.Printf("\tgenerating validation method\n")

			fmt.Fprintln(out, "func (in *"+currType.Name.Name+") validate(data []byte) error {")
			fmt.Fprintln(out, "	r := bytes.NewReader(data)")

		FIELDS_LOOP:
			for _, field := range currStruct.Fields.List {
				if field.Tag != nil {
					//fmt.Printf(" analyze field: %v", field)
					tag := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])
					if tag.Get("apivalidator") == "" {
						continue FIELDS_LOOP
					}

					fieldName := field.Names[0].Name
					fileType := field.Type.(*ast.Ident).Name

					fmt.Printf("\tgenerating code for field %s.%s\n", currType.Name.Name, fieldName)

					switch fileType {
					case "int":
						intTpl.Execute(out, tpl{fieldName})
					case "string":
						strTpl.Execute(out, tpl{fieldName})
					default:
						log.Fatalln("unsupported", fileType)
					}
				}
			}

			fmt.Fprintln(out, "	return nil")
			fmt.Fprintln(out, "}") // end of Unpack func
			fmt.Fprintln(out)      // empty line

		}
	}
}
