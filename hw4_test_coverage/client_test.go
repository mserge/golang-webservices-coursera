package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type Row struct {
	Id string `xml:"id"`
}

type Root struct {
	Version string `xml:"version,attr"`
	List    []Row  `xml:"row"`
}

type SearchResult struct {
	NumUsers int
	NextPage bool
}

type TestCase struct {
	ID      string // goes to token
	Request *SearchRequest
	Result  *SearchResult
	IsError bool
}

// код писать тут
func loadXML(fileName string) (*Root, error) {
	xmlFile, err := os.Open(fileName)
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	fmt.Sprintf("Successfully Opened %s", fileName)
	// defer the closing of our xmlFile so that we can parse it later on
	defer xmlFile.Close()

	// read our opened xmlFile as a byte array.
	byteValue, _ := ioutil.ReadAll(xmlFile)
	v := new(Root)
	err = xml.Unmarshal(byteValue, &v)
	if err != nil {
		fmt.Printf("error: %v", err)
		return nil, err
	}
	return v, nil
}

func newSearchHandler(root *Root) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		log.Printf("Called %s", r.URL)
		log.Printf(r.Header.Get("AccessToken"))
		bytes, e := json.Marshal(root.List)
		if e != nil {
			log.Fatal(e)
			rw.WriteHeader(http.StatusInternalServerError)
		} else {
			rw.WriteHeader(http.StatusOK)
			rw.Write(bytes)

		}
	}
}

func TestSearchClient_FindUsers(t *testing.T) {
	root, e := loadXML("dataset.xml")
	println("%v %e", &root, e)

	ts := httptest.NewServer(http.HandlerFunc(newSearchHandler(root)))
	cases := []TestCase{
		TestCase{
			ID:      "",
			Request: &SearchRequest{Limit: -1},
			Result:  &SearchResult{0, false},
			IsError: true,
		},
	}
	for caseNum, item := range cases {

		searchClient := SearchClient{item.ID, ts.URL}
		request := SearchRequest{}
		result, err := searchClient.FindUsers(request)
		if err != nil && !item.IsError {
			t.Errorf("[%d] unexpected error: %#v", caseNum, err)
		}
		if err == nil && item.IsError {
			t.Errorf("[%d] expected error, got nil", caseNum)
		}
		if compareResult(result, item.Result) {
			t.Errorf("[%d] wrong result, expected %#v, got %#v", caseNum, item.Result, result)
		}
	}
	ts.Close()

}

func compareResult(response *SearchResponse, result *SearchResult) bool {
	return response != nil && response.NextPage == result.NextPage && len(response.Users) == result.NumUsers
}
