package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

//easyjson:json
type User struct {
	Browsers []string `json:"browsers"`
	//Company  string   `json:"company"`
	//Country  string   `json:"country"`
	Email string `json:"email"`
	//Job      string   `json:"job"`
	Name string `json:"name"`
	//Phone    string   `json:"phone"`
}

// вам надо написать более быструю оптимальную этой функции
func FastSearch(out io.Writer) {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}

	matchBrowsers := []string{"Android", "MSIE"}
	seenBrowsers := make(map[string]bool, 128) // preallocate for 128 browsers
	i := -1                                    // headers
	scanner := bufio.NewScanner(file)
	user := User{}
	fmt.Fprint(out, "found users:\n")

USERS:
	for scanner.Scan() {
		i++
		err := user.UnmarshalJSON(scanner.Bytes())
		if err != nil {
			panic(err)
		}
		matched := []bool{false, false}
		//BROWSERS:
		for _, browser := range user.Browsers {
			for i, match := range matchBrowsers {
				if strings.Contains(browser, match) {
					matched[i] = true
					seenBrowsers[browser] = true
				}
			}
			if matched[0] && matched[1] { // at some point we found - no need to go further - can be many AND
				// log.Println("Android and MSIE user:", user["name"], user["email"])
				fmt.Fprintf(out, "[%d] %s <%s>\n", i, user.Name, strings.Replace(user.Email, "@", " [at] ", 1))
				continue USERS
			}
		}

	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
	fmt.Fprintln(out, "\nTotal unique browsers", len(seenBrowsers))
}

/*
BenchmarkSlow-8               10         102341950 ns/op        319475647 B/op    276175 allocs/op
BenchmarkFast-8              500           2461296 ns/op          487200 B/op       6425 allocs/op
											41							655				42
BenchmarkSlow-8               10         124178270 ns/op        319475184 B/op    276177 allocs/op
BenchmarkFast-8              500           2717256 ns/op          487204 B/op       6425 allocs/op

											45
BenchmarkSolution							51							601				27
BenchmarkSlow-8 10 							142703250 ns/op 		336887900 B/op 284175 allocs/op
BenchmarkSolution-8 500 					2782432 ns/op 			559910 B/op 	10422 allocs/op
*/
