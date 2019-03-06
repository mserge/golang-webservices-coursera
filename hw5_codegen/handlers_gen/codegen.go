package main

import (
	"encoding/json"
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

type FieldTpl struct {
	FieldName string
	ParamName string
}

type DefaultFieldTpl struct {
	FieldTpl
	DefaultValue string
}

type EnumFieldTpl struct {
	FieldTpl
	AllowedValues []string
}

type EndPoint struct {
	FuncName string
	URL      string `json:"url"`
	Auth     bool   `json:"auth"`
	Method   string `json:"method"`
}

type ApiTpl struct {
	HandlerName string
	EndPoints   []EndPoint
}

var (
	intTpl = template.Must(template.New("intTpl").Parse(`
	// int {{.FieldName}}
	 in.{{.FieldName}}, err = strconv.Atoi(r.FormValue("{{.ParamName}}"))
	if err != nil {
		return ApiError{ http.StatusBadRequest, fmt.Errorf("{{.ParamName}} must be int")}
	}
`))

	strTpl = template.Must(template.New("strTpl").Parse(`
	// string {{.FieldName}}
	in.{{.FieldName}} = r.FormValue("{{.ParamName}}")
`))
	requiredTpl = template.Must(template.New("requiredTpl").Parse(`
	// required {{.FieldName}}
	if r.FormValue("{{.ParamName}}") == "" {
		return ApiError{ http.StatusBadRequest, fmt.Errorf("{{.ParamName}} must me not empty")}
	}
`))
	defaultTpl = template.Must(template.New("defaultTpl").Parse(`
	// default {{.FieldName}}
	if r.FormValue("{{.ParamName}}") == "" {
		r.Form.Set("{{.ParamName}}" ,"{{.DefaultValue}}")
	}
`))

	enumTpl = template.Must(template.New("enumTpl").Parse(`
	// enum {{.FieldName}}
	switch  r.FormValue("{{.ParamName}}") {
		case {{range $i,$a := .AllowedValues}} {{if gt $i 0 }} , {{end}} "{{.}}" {{end}}:
		default:
		return ApiError{ http.StatusBadRequest, fmt.Errorf("{{.ParamName}} must be one of [{{range $i,$a := .AllowedValues}}{{if gt $i 0 }}, {{end}}{{.}}{{end}}]")}

	}
`))
	serveTpl = template.Must(template.New("serveTpl").Parse(`
// {{.HandlerName}}
func (h *{{.HandlerName}} ) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
		{{range .EndPoints}} 
		case "{{.URL}}": 
			methodMiddleware("{{.Method}}", authMiddleware({{.Auth}}, h.handler{{.FuncName}}())).ServeHTTP(w, r)
		{{end}}
    default:
        // 404
		writeResponse(w, nil , ApiError{http.StatusNotFound, fmt.Errorf("unknown method")})
    }
}
`))

	funcs = `
type  APIResponse struct{
	Response interface{} ` + "`" + `json:"response,omitempty"` + "`" + `
	Error string ` + "`" + `json:"error"` + "`" + `
}

func  writeResponse(w http.ResponseWriter, response interface{}, err error){
	data := APIResponse{}
	if err != nil {
		data.Error = err.Error()
		apierror, ok := err.(ApiError)
		if(ok){
			w.WriteHeader(apierror.HTTPStatus)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
 	} else {
		w.WriteHeader(http.StatusOK)
 		data.Response = response
	}
	resencode, err := json.Marshal(data)
	if err == nil {
		w.Write(resencode)
	}
}
func methodMiddleware(method string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if method != "" && r.Method !=  method {
			writeResponse(w, nil, ApiError{406, fmt.Errorf("bad method")})
		} else {
			next.ServeHTTP(w, r)
		}

	})
}
func authMiddleware(required bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if required && r.Header.Get("X-Auth") !=  "100500" {
			writeResponse(w, nil, ApiError{403, fmt.Errorf("unauthorized")})
		} else {
			next.ServeHTTP(w, r)
		}

	})
}
`
)

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, _ := os.Create(os.Args[2])
	apis := make(map[string]ApiTpl)
	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, `import "net/http"`)
	fmt.Fprintln(out, `import "fmt"`)
	fmt.Fprintln(out, `import "encoding/json"`)
	fmt.Fprintln(out, `import "strconv"`)
	fmt.Fprintln(out, funcs) // empty line

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
		endpoint := EndPoint{FuncName: fun.Name.Name}
		err := json.Unmarshal([]byte(jsonStruct), &endpoint)
		if err != nil {
			fmt.Printf("SKIP func %#v have incorrect tag %s: %s\n", fun.Name.Name, err, jsonStruct)
			continue FUNC_LOOP

		}
		if field := fun.Recv.List[0]; field != nil {
			if star, ok := field.Type.(*ast.StarExpr); ok {
				if ident, ok := star.X.(*ast.Ident); ok {
					name = ident.Name
					fmt.Printf("need to generate type %s\n", name)
					api, ok := apis[name]
					if !ok { // if not exists
						points := make([]EndPoint, 1)
						points[0] = endpoint
						api = ApiTpl{name, points}
						api.HandlerName = name
					} else {
						api.EndPoints = append(api.EndPoints, endpoint)
					}
					apis[name] = api

					fmt.Fprintf(out, ` func (h *%s ) handler%s() http.Handler {
	 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
`, name, fun.Name.Name)
					// заполнение структуры params
					params := make([]string, 0, 1)
					for _, paramfield := range fun.Type.Params.List {
						fieldType := paramfield.Type
						fmt.Printf("field names; %#v , type: %#v \n", fieldType)
						ft, ftok := fieldType.(*ast.Ident)
						for _, fieldname := range paramfield.Names {
							if ftok {
								params = append(params, fieldname.Name)
								fmt.Fprintf(out, " 		%s := %s{} \n", fieldname.Name, ft.Name)
								// валидирование параметров
								fmt.Fprintf(out, " 		err := %s.validate(r)\n", fieldname.Name)
								fmt.Fprintln(out, `		var res interface{}
	 	if err == nil {`)
							}
						}
					}
					fmt.Fprintf(out, ` 			res, err = h.%s(r.Context(), %s)
	`, fun.Name.Name, strings.Join(params, ","))
					// прочие обработки
					fmt.Fprintln(out, ` 	}
	 	writeResponse(w, res, err)	
	})
}
`)
				}
			}
		}

	}

	// generate all wrappers calls on the type
	for key, api := range apis {
		fmt.Printf("WILL generate %s : %#v\n", key, api)
		serveTpl.Execute(out, api)
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

			fmt.Fprintln(out, "func (in *"+currType.Name.Name+") validate(r *http.Request) error {")
			fmt.Fprintln(out, `	    err := r.ParseForm()
			if err != nil {
				// Handle error
				return err
			}`)

		FIELDS_LOOP:
			for _, field := range currStruct.Fields.List {
				if field.Tag != nil {
					//fmt.Printf(" analyze field: %v", field)
					tag := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])
					apiValidatorTag := tag.Get("apivalidator")
					if apiValidatorTag == "" {
						continue FIELDS_LOOP
					}

					fieldName := field.Names[0].Name
					fileType := field.Type.(*ast.Ident).Name

					fmt.Printf("\tgenerating code for field %s.%s with validation: %s\n", currType.Name.Name, fieldName, apiValidatorTag)
					options := strings.Split(apiValidatorTag, ",")
					values := make(map[string]string)
					fieldTpl := FieldTpl{fieldName, strings.ToLower(fieldName)}
					for _, option := range options {
						if strings.Contains(option, "=") {
							option_args := strings.Split(option, "=")
							values[option_args[0]] = option_args[1]
						} else {
							values[option] = "set"
						}
					}
					if values["paramname"] != "" {
						fieldTpl.ParamName = values["paramname"]
					}
					if values["default"] != "" {
						defaultTpl.Execute(out, DefaultFieldTpl{fieldTpl, values["default"]})
					} else if values["required"] != "" { // default value
						requiredTpl.Execute(out, fieldTpl)
					}
					if values["enum"] != "" {
						enumTpl.Execute(out, EnumFieldTpl{fieldTpl, strings.Split(values["enum"], "|")})
					}
					switch fileType {
					case "int":
						intTpl.Execute(out, fieldTpl)
					case "string":
						strTpl.Execute(out, fieldTpl)
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
