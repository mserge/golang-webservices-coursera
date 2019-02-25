package main

import (
	"fmt"
	"io"
	"os"
	"sort"
)

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}

const LEVEL_LAST_PREFIX = "	"
const LEVEL_MIDDLE_PREFIX = "│	"
const LEVEL_ITEM_LAST_PREFIX = "└───"
const LEVEL_ITEM_PREFIX = "├───"

func printTree(out io.Writer, path string, printFiles bool, prefix string) error {
	file, e := os.Open(path)
	if e != nil {
		return e
	}
	infos, e := file.Readdir(-1)
	// TODO remove files from infos inplace https://stackoverflow.com/a/20551116/5892568
	i := 0 // output index
	for _, x := range infos {
		if x.IsDir() || printFiles {
			// copy and increment index
			infos[i] = x
			i++
		}
	}
	infos = infos[:i]

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name() < infos[j].Name()
	})

	for i, info := range infos {

		s := ""                // if we're on level add
		if i == len(infos)-1 { // last
			s = prefix + LEVEL_ITEM_LAST_PREFIX
		} else {
			s = prefix + LEVEL_ITEM_PREFIX
		}
		s += info.Name()
		if !info.IsDir() {
			if info.Size() == 0 {
				s += " (empty)"
			} else {
				s += fmt.Sprintf(" (%db)", info.Size())
			}
		}
		s += "\n"
		_, err := out.Write([]byte(s))
		if err != nil {
			return err
		}
		if info.IsDir() {
			newprefix := prefix
			if i == len(infos)-1 { // last
				newprefix += LEVEL_LAST_PREFIX
			} else {
				newprefix += LEVEL_MIDDLE_PREFIX
			}
			err = printTree(out, path+string(os.PathSeparator)+info.Name(), printFiles, newprefix)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func dirTree(out io.Writer, path string, printFiles bool) error {
	return printTree(out, path, printFiles, "")
}
