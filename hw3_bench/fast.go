package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

// вам надо написать более быструю оптимальную этой функции
func FastSearch(out io.Writer) {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}

	fileContents, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}

	matchBrowsers := []string{"Android", "MSIE"}
	seenBrowsers := make(map[string]bool, 1000) // preallocate for 1000 browsers
	foundUsers := ""
	r := regexp.MustCompile("@")
	// TODO Buffer
	lines := strings.Split(string(fileContents), "\n")
	i := -1 // headers
USERS:
	for _, line := range lines {
		i++
		user := make(map[string]interface{})
		// fmt.Printf("%v %v\n", err, line)
		err := json.Unmarshal([]byte(line), &user)
		if err != nil {
			panic(err)
		}
		matched := []bool{false, false}
		browsers, ok := user["browsers"].([]interface{})
		if !ok {
			// log.Println("cant cast browsers")
			continue
		}
		//BROWSERS:
		for _, browserRaw := range browsers {
			browser, ok := browserRaw.(string)
			if !ok {
				// log.Println("cant cast browser to string")
				continue
			}
			for i, match := range matchBrowsers {
				if strings.Contains(browser, match) {
					matched[i] = true
					seenBrowsers[browser] = true
				}
			}
			if matched[0] && matched[1] { // at some point we found - no need to go further - can be many AND
				// log.Println("Android and MSIE user:", user["name"], user["email"])
				email := r.ReplaceAllString(user["email"].(string), " [at] ")
				foundUsers += fmt.Sprintf("[%d] %s <%s>\n", i, user["name"], email)
				continue USERS
			}
		}

	}

	fmt.Fprintln(out, "found users:\n"+foundUsers)
	fmt.Fprintln(out, "Total unique browsers", len(seenBrowsers))
}
