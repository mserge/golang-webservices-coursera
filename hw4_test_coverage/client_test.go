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
	"strconv"
	"testing"
	"time"
)

type Row struct {
	Id     int    `xml:"id"`
	Name   string `xml:"first_name"` // no matter if we load only first here
	Age    int    `xml:"age"`
	About  string `xml:"about"`
	Gender string `xml:"gender"`
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
		//log.Printf("Called %s", r.URL)
		at := r.Header.Get("AccessToken")
		limit, _ := strconv.ParseInt(r.FormValue("limit"), 10, 0)
		offset, _ := strconv.ParseInt(r.FormValue("offset"), 10, 0)
		last := int(offset + limit)
		if last > len(root.List) {
			last = len(root.List)
		}
		bytes, e := json.Marshal(root.List[offset:last])
		if e != nil {
			log.Fatal(e)
			rw.WriteHeader(http.StatusInternalServerError)
		} else {
			switch at {
			case "bad":
				rw.WriteHeader(http.StatusUnauthorized)
				rw.Write(bytes)
			case "jsonerror":
				rw.WriteHeader(http.StatusOK)
				rw.Write([]byte("nonjson"))
			case "badreq-json":
				rw.WriteHeader(http.StatusBadRequest)
				rw.Write([]byte("nonjson"))
			case "badreq-field":
				rw.WriteHeader(http.StatusBadRequest)
				ser, _ := json.Marshal(SearchErrorResponse{ErrorBadOrderField})
				rw.Write(ser)
			case "badreq-field-id":
				rw.WriteHeader(http.StatusBadRequest)
				ser, _ := json.Marshal(SearchErrorResponse{"ErrorBadOrderField"})
				rw.Write(ser)
			case "unknown":
				rw.Header().Set("Location", "")
				rw.WriteHeader(http.StatusMovedPermanently)
				// nothing
			case "error":
				rw.WriteHeader(http.StatusInternalServerError)
			case "timeout":
				time.Sleep(2 * time.Second)
				fallthrough
			default:
				rw.WriteHeader(http.StatusOK)
				rw.Write(bytes)
			}

		}
	}
}

func (sr *SearchResult) Stringer() string {
	res := fmt.Sprintf("%d users ", sr.NumUsers)
	if sr.NextPage {
		res += "with next page"
	}
	return res
}

func TestSearchClient_FindUsers(t *testing.T) {
	root, e := loadXML("dataset.xml")
	if e != nil {
		t.Error("XML not loaded", e)
	}
	ts := httptest.NewServer(http.HandlerFunc(newSearchHandler(root)))
	cases := []TestCase{
		TestCase{
			ID:      "ok",
			Request: &SearchRequest{Limit: 25, Offset: 25},
			Result:  &SearchResult{10, false},
			IsError: false,
		},
		TestCase{
			ID:      "limit-1",
			Request: &SearchRequest{Limit: -1},
			Result:  &SearchResult{0, false},
			IsError: true,
		},
		TestCase{
			ID:      "limitmore25",
			Request: &SearchRequest{Limit: 26},
			Result:  &SearchResult{25, true},
			IsError: false,
		},
		TestCase{
			ID:      "offset-1",
			Request: &SearchRequest{Limit: 1, Offset: -1},
			Result:  &SearchResult{0, false},
			IsError: true,
		},
		TestCase{
			ID:      "bad",
			Request: &SearchRequest{Limit: 1, Offset: 0},
			Result:  &SearchResult{1, true},
			IsError: true,
		},
		TestCase{
			ID:      "unknown",
			Request: &SearchRequest{Limit: 1, Offset: 0},
			Result:  &SearchResult{1, true},
			IsError: true,
		},
		TestCase{
			ID:      "error",
			Request: &SearchRequest{Limit: 1, Offset: 0},
			Result:  &SearchResult{1, true},
			IsError: true,
		},
		TestCase{
			ID:      "jsonerror",
			Request: &SearchRequest{Limit: 1, Offset: 0},
			Result:  &SearchResult{1, true},
			IsError: true,
		},
		TestCase{
			ID:      "badreq-json",
			Request: &SearchRequest{Limit: 1, Offset: 0},
			Result:  &SearchResult{1, true},
			IsError: true,
		},
		TestCase{
			ID:      "badreq-field",
			Request: &SearchRequest{Limit: 1, Offset: 0},
			Result:  &SearchResult{1, true},
			IsError: true,
		},
		TestCase{
			ID:      "badreq-field-id",
			Request: &SearchRequest{Limit: 1, Offset: 0, OrderField: "xxx"},
			Result:  &SearchResult{1, true},
			IsError: true,
		},
		TestCase{
			ID:      "timeout",
			Request: &SearchRequest{Limit: 1, Offset: 0},
			Result:  &SearchResult{0, false},
			IsError: true,
		},
	}

	for caseNum, item := range cases {
		// https://blog.golang.org/subtests
		testcase := fmt.Sprintf("Case #%s with req %+v expected %v result ", item.ID, item.Request, item.Result)
		if item.IsError {
			testcase += "with error"
		} else {
			testcase += "success"
		}
		t.Run(testcase, func(t *testing.T) {
			searchClient := SearchClient{item.ID, ts.URL}
			request := item.Request
			result, err := searchClient.FindUsers(*request)
			if err != nil && !item.IsError {
				t.Errorf("[%d] unexpected error: %#v", caseNum, err)
			}
			if err == nil && item.IsError {
				t.Errorf("[%d] expected error, got nil", caseNum)
			}
			if err == nil && !item.IsError && !compareResult(result, item.Result) {
				t.Errorf("[%d] wrong result, expected %#v, got %#v", caseNum, item.Result, result)
			}
		})
	}
	ts.Close()

}

func compareResult(response *SearchResponse, result *SearchResult) bool {
	return response != nil && response.NextPage == result.NextPage && len(response.Users) == result.NumUsers
}
