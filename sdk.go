// sdk
package main

//known bug: the __DIR__ and __FILE__ variables should be []string{} not string because because they will vary depending on what script is running in the main go routine.

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/olebedev/go-duktape.v3"

	"github.com/niubaoshu/gotiny"
)

type dukValue struct {
	value *duktape.Context
}

func Run(duk *duktape.Context, input string) (dukValue, error) {

	//	indexStart := 0

	//	for len(input) > indexStart {
	//		i1 := strings.Index(input[indexStart+1:], `"`)
	//		i2 := strings.Index(input[indexStart+1:], `'`)
	//		i3 := strings.Index(input[indexStart+1:], "'")

	//	}

	e := duk.PevalString(input)
	dv := dukValue{}

	if e == nil {

		dv.value = duk

		return dv, nil
	} else {
		return dv, e
	}

}

var routinesTriggered bool = false
var callIncrement float64 = 0.0000000000000000000000000001
var cacheChanged bool = false
var globalChanged bool = false
var cacheMBSize float64 = 20.0
var isdevmode bool = false
var forceCache bool = false
var noCache bool = true
var globalvm []*duktape.Context = make([]*duktape.Context, 0)
var mutex sync.RWMutex
var maxExecutionTime time.Duration
var singletonTrigger bool = false
var tempNoCache bool = true
var tempForceCache bool = false
var tempForceDevMode bool = false

func httpGet(url string) string {

	// don't worry about errors
	response, e := http.Get(url)

	if e != nil {
		return ""
	}
	defer response.Body.Close()

	html, err := ioutil.ReadAll(response.Body)

	if err != nil {
		return ""
	}

	// show the HTML code as a string %s
	return string(html)

}

type cacheVal struct {
	val   string
	calls float64
}

var cachedScripts map[string]*cacheVal = make(map[string]*cacheVal, 0)
var __DIR__ string = os.Args[0]
var __FILE__ string = os.Args[0]

var preCachedString []string = make([]string, len(os.Args))

func encodeCache() []byte {

	//	b := new(bytes.Buffer)
	//	e := gob.NewEncoder(b)
	//	// Encoding the map
	//	err := e.Encode(cachedScripts)
	//	if err != nil {

	//	}

	//	return b.Bytes()

	return gotiny.Marshal(&cachedScripts)
}

func vmSetStringVar(ctx *duktape.Context, name string, variable string) {

	b, e := json.Marshal(variable)
	if e == nil {
		ctx.PevalString("var " + name + " = " + `JSON.parse("` + strings.Replace(string(b), `"`, `\"`, -1) + `");`)
	}

}

func encodeGlobal() []byte {
	//	b := new(bytes.Buffer)
	//	e := gob.NewEncoder(b)
	//	// Encoding the map
	//	err := e.Encode(glob)
	//	if err != nil {

	//	}

	//	return b.Bytes()

	return gotiny.Marshal(&glob)
}

func decodeCache(data []byte) {
	gotiny.Unmarshal(data, &cachedScripts)
	//	if !isdevmode {

	//		b := new(bytes.Buffer)
	//		_, err := b.Write(data)

	//		if err == nil {
	//			var decodedMap map[string]string

	//			d := gob.NewDecoder(b)

	//			// Decoding the serialized data
	//			err = d.Decode(&decodedMap)
	//			if err != nil {

	//			} else {
	//				// Ta da! It is a map!
	//				cachedScripts = decodedMap
	//			}
	//		}

	//	}

}

func decodeGlobal(data []byte) {
	gotiny.Unmarshal(data, &glob)
	//	if !isdevmode {
	//		b := new(bytes.Buffer)
	//		_, err := b.Write(data)
	//		if err == nil {
	//			var decodedMap map[string]interface{}

	//			d := gob.NewDecoder(b)

	//			// Decoding the serialized data
	//			err = d.Decode(&decodedMap)
	//			if err != nil {

	//			} else {
	//				// Ta da! It is a map!
	//				glob = decodedMap
	//			}
	//		}
	//	}
}

var readExecuteTOdoLiveMode []bool = make([]bool, 0)
var readExecuteTOdoNotCache []bool = make([]bool, 0)
var readExecuteTODOpath []string = make([]string, 0)
var readExecuteTODOargsString []string = make([]string, 0)
var readExecuteTODOOttoNum []int = make([]int, 0)

func convertRegex(sentence string) string {

	return "(^|\\s+)+" + "(" + strings.Replace(sentence, " ", "(^|\\s+|$)+", -1) + ")" + "($|\\s+)+"
}
func testIfShouldLiveUpdate(statement string) bool {
	liveRegex := "*make*live*|*set*live*|*publish*live*|*update*live*"
	if pass, e := regexpMatchString(`*do*not*make*live*|*don?t*make*live*|do*not*set*live*|don?t*set*live*|don?t*publish*live*|do*not*publish*live*|don?t*update*live*|do*not*update*live*`, (statement)); e == nil && pass {

		return false

	} else if pass, e := regexpMatchString(liveRegex, statement); e == nil && pass {

		return true

	}

	return isdevmode
}
func testIfShouldNOTCache(statement string) (bool, bool) {
	if pass, e := regexpMatchString(("*do*not*cache*|*don?t*cache*|*force*no*cache*"), strings.ToLower(statement)); e == nil && pass {
		return true, false

	} else if pass, e := regexpMatchString(("*do*cache*|set*cache*|force*cache*|*cache*"), strings.ToLower(statement)); e == nil && pass {
		return false, true

	}

	return noCache, forceCache
}

func testRequest(x int, paths []string, args []string, gets map[string]string, posts map[string]string, doNotCache bool, variables map[string]string, ottoNum int) (bool, int, bool, map[string]string) {
	lastVariableKey := ""
	subjectText := ""
	objectText := ""
	returnTrue := true

	statementFound := false
	endX := x

	for x > 0 && len(paths) > x {

		for varkey, varval := range variables {

			paths[x] = strings.Replace(paths[x], varkey, varval, -1)
		}

		statementFound = false
		statement := strings.ToLower(paths[x-1])
		if pass, e := regexpMatchString(`*always*|*no\smatter\swhat*|*every\stime*|*if\strue*`, statement); e == nil && pass {

			statementFound = false // this is a special case <<<<---
			returnTrue = true

		} else if pass, e := regexpMatchString((`*else\sif*|*set*|*replace*|*change*|*if*|*write*`), statement); e == nil && pass {
			statementFound = true
			//			fmt.Println(paths[x])
			//fmt.Println(statement)
			//e.g. if arg kesy equals
			// else
			if strings.Contains(statement, "set") || strings.Contains(statement, "replace") || strings.Contains(statement, "change") {

				lastVariableKey = paths[x]
			} else if pass2, _ := regexpMatchString("*global*", statement); pass2 {

				subjectText = ""
				if ans, ok := glob[paths[x]]; ok {
					subjectText = ans
					lastVariableKey = subjectText
					objectText = subjectText

				} else {
					subjectText = ""
					lastVariableKey = subjectText
					objectText = subjectText

				}

			} else if pass2, _ := regexpMatchString("*arg*", statement); pass2 {

				subjectText = ""

				for val, ans := range args {
					v := strconv.Itoa(val)
					if paths[x] == v {
						if pass, e := regexpMatchString(("*file*name*|*file*"), statement); e == nil && pass {
							subjectText = filepath.Base(ans)
						} else if pass, e := regexpMatchString(("*dir*path*|*folder*path*|*parent*path*"), statement); e == nil && pass {
							subjectText = filepath.Dir(ans)
						} else if pass, e := regexpMatchString(("*dir*name*|*folder*name*|*parent*name*"), statement); e == nil && pass {
							subjectText = filepath.Base(filepath.Dir(ans))
						} else {
							subjectText = ans
						}

						lastVariableKey = subjectText
						objectText = subjectText
						// strings.Replace(strings.Replace(strings.Replace(ans, "\\\\", "odjs,dsam2eiuh", -1), " ", "\\ ", -1), "odjs,dsam2eiuh", "\\", -1)
						//						fmt.Println(subjectText)
					}
				}

			} else if strings.Contains(statement, " post ") {

				subjectText = ""
				objectText = paths[x]
				for val, ans := range posts {
					if paths[x] == val {

						subjectText = ans

						lastVariableKey = subjectText
						objectText = subjectText
					}
				}

			} else if strings.Contains(statement, " get ") {

				subjectText = ""
				objectText = paths[x]
				for val, ans := range gets {
					if paths[x] == val {
						subjectText = ans

						lastVariableKey = subjectText
						objectText = subjectText
					}
				}

			} else if strings.Contains(statement, " request ") {
				subjectText = ""
				objectText = paths[x]
				for val, ans := range posts {
					if paths[x] == val {
						subjectText = ans

						lastVariableKey = subjectText
						objectText = subjectText
					}
				}
				if subjectText == "" {
					for val, ans := range gets {
						if paths[x] == val {
							subjectText = ans

							lastVariableKey = subjectText
							objectText = subjectText
						}
					}
				}

			} else if pass, e := regexpMatchString(`*anything*|*any\sthing*|*any-arg*|*arg*|*any-key*|*any\sarg*|*any\skey*`, statement); e == nil && pass {
				subjectText = ""
				objectText = paths[x]
				for val, ans := range posts {
					if paths[x] == val {
						subjectText = ans

						lastVariableKey = subjectText
						objectText = subjectText
					}
				}
				if subjectText == "" {
					for val, ans := range gets {
						if paths[x] == val {
							subjectText = ans

							lastVariableKey = subjectText
							objectText = subjectText
						}
					}

					if subjectText == "" {
						for val, ans := range args {
							v := strconv.Itoa(val)
							if paths[x] == v {

								if pass, e := regexpMatchString(("*file*name*|*file*"), statement); e == nil && pass {
									subjectText = filepath.Base(ans)
								} else if pass, e := regexpMatchString(("*dir*path*|*folder*path*|*parent*path*"), statement); e == nil && pass {
									subjectText = filepath.Dir(ans)
								} else if pass, e := regexpMatchString(("*dir*name*|*folder*name*|*parent*name*"), statement); e == nil && pass {
									subjectText = filepath.Base(filepath.Dir(ans))
								} else {
									subjectText = ans
								} // strings.Replace(strings.Replace(strings.Replace(ans, "\\\\", "odjs,dsam2eiuh", -1), " ", "\\ ", -1), "odjs,dsam2eiuh", "\\", -1)
								//								fmt.Println(subjectText)

								lastVariableKey = subjectText
								objectText = subjectText
							}
						}
					}

				}

			} else {
				objectText = paths[x]
				subjectText = paths[x]
				lastVariableKey = subjectText
			}

			if pass, e := regexpMatchString(`*js*|*eval*|*javascript*`, statement); e == nil && pass {

				val, _ := Run(globalvm[ottoNum], lastVariableKey)

				lastVariableKey = val.value.GetString(-1)
				objectText = lastVariableKey
				subjectText = lastVariableKey

			}

		} else if pass, e := regexpMatchString((`*unset*|*remove*|*delete*|*clear*`), statement); e == nil && pass {
			delete(variables, paths[x])
			statementFound = true // this is a special case <<<<---
		} else if pass, e := regexpMatchString(`*else*`, statement); e == nil && pass {

			statementFound = false // this is a special case <<<<---
			returnTrue = !returnTrue

		} else if pass, e := regexpMatchString("*does*not*equal*|*!=*|*<>*|*not*equal*|*doesn?t*equal*", statement); e == nil && pass && objectText != "" {

			statementFound = true

			if pass2, _ := regexpMatchString("*global*", statement); pass2 {

				if ans, ok := glob[paths[x]]; ok {
					objectText = ans
					subjectText = fmt.Sprint(ans)
				}

			} else if pass2, _ := regexpMatchString("*arg*", statement); pass2 {

				for val, ans := range args {
					v := strconv.Itoa(val)
					if paths[x] == v {
						objectText = subjectText
						if pass, e := regexpMatchString(("*file*name*|*file*"), statement); e == nil && pass {
							subjectText = filepath.Base(ans)
						} else if pass, e := regexpMatchString(("*dir*path*|*folder*path*|*parent*path*"), statement); e == nil && pass {
							subjectText = filepath.Dir(ans)
						} else if pass, e := regexpMatchString(("*dir*name*|*folder*name*|*parent*name*"), statement); e == nil && pass {
							subjectText = filepath.Base(filepath.Dir(ans))
						} else {
							subjectText = ans
						}
						// strings.Replace(strings.Replace(strings.Replace(ans, "\\\\", "odjs,dsam2eiuh", -1), " ", "\\ ", -1), "odjs,dsam2eiuh", "\\", -1)
						//						fmt.Println(subjectText)
					}
				}

			} else if strings.Contains(statement, " post ") {

				for val, ans := range posts {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}

			} else if strings.Contains(statement, " get ") {

				for val, ans := range gets {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}

			} else if strings.Contains(statement, " request ") {

				for val, ans := range posts {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}
				if subjectText == "" {
					for val, ans := range gets {
						if paths[x] == val {
							objectText = subjectText
							subjectText = ans
						}
					}
				}

			} else {
				objectText = subjectText
				subjectText = paths[x]
			}

			if pass, e := regexpMatchString(`*js*|*eval*|*javascript*`, statement); e == nil && pass {

				val, _ := Run(globalvm[ottoNum], subjectText)

				subjectText = val.value.GetString(-1)

			}

			if objectText != subjectText {
				returnTrue = true
			} else {
				returnTrue = false
				//return false, x+2, doNotCache, variables
			}
		} else if pass, e := regexpMatchString(("*equals*|*=*|*==*"), statement); e == nil && pass && objectText != "" {
			statementFound = true

			if pass2, _ := regexpMatchString("*global*", statement); pass2 {

				if ans, ok := glob[paths[x]]; ok {
					objectText = subjectText
					subjectText = ans

				} else {
					objectText = subjectText
					subjectText = ""
				}

			} else if pass2, _ := regexpMatchString("*arg*", statement); pass2 {
				objectText = subjectText
				subjectText = ""
				for val, ans := range args {
					v := strconv.Itoa(val)
					if paths[x] == v {

						if pass, e := regexpMatchString(("*file*name*|*file*"), statement); e == nil && pass {
							subjectText = filepath.Base(ans)
						} else if pass, e := regexpMatchString(("*dir*path*|*folder*path*|*parent*path*"), statement); e == nil && pass {
							subjectText = filepath.Dir(ans)
						} else if pass, e := regexpMatchString(("*dir*name*|*folder*name*|*parent*name*"), statement); e == nil && pass {
							subjectText = filepath.Base(filepath.Dir(ans))
						} else {
							subjectText = ans
						}
						// strings.Replace(strings.Replace(strings.Replace(ans, "\\\\", "odjs,dsam2eiuh", -1), " ", "\\ ", -1), "odjs,dsam2eiuh", "\\", -1)
						//						fmt.Println(subjectText)
					}
				}

			} else if strings.Contains(statement, " post ") {
				objectText = subjectText
				subjectText = ""
				for val, ans := range posts {
					if paths[x] == val {

						subjectText = ans
					}
				}

			} else if strings.Contains(statement, " get ") {
				objectText = subjectText
				subjectText = ""
				for val, ans := range gets {
					if paths[x] == val {

						subjectText = ans
					}
				}

			} else if strings.Contains(statement, " request ") {
				objectText = subjectText
				subjectText = ""
				for val, ans := range posts {
					if paths[x] == val {

						subjectText = ans
					}
				}
				if subjectText == "" {
					objectText = subjectText
					subjectText = ""
					for val, ans := range gets {
						if paths[x] == val {

							subjectText = ans
						}
					}
				}

			} else {
				objectText = subjectText
				subjectText = paths[x]
			}

			if pass, e := regexpMatchString(`*js*|*eval*|*javascript*`, statement); e == nil && pass {

				val, _ := Run(globalvm[ottoNum], subjectText)

				subjectText = val.value.GetString(-1)

			}

			if objectText == subjectText {
				returnTrue = true
			} else {
				returnTrue = false
				//return false, x+2, doNotCache, variables
			}

		} else if pass, e := regexpMatchString(("*match*not*perfect*|*regex*not*perfect*|*not*match*perfect*|*not*perfect*regex*|*not*perfect*match*|*doesn?t*perfect*match*|*regex*not*complete*|*not*match*complete*|*not*complete*regex*|*not*complete*match*|*doesn?t*complete*match*|*doesn?t*match*perfectly*"), statement); e == nil && pass && objectText != "" {
			//			fmt.Println(statement)
			statementFound = true
			if pass2, _ := regexpMatchString("*global*", statement); pass2 {

				if ans, ok := glob[paths[x]]; ok {
					objectText = ans
					subjectText = fmt.Sprint(ans)
				}

			} else if pass2, _ := regexpMatchString("*arg*", statement); pass2 {

				for val, ans := range args {
					v := strconv.Itoa(val)
					if paths[x] == v {
						objectText = subjectText
						if pass, e := regexpMatchString(("*file*name*|*file*"), statement); e == nil && pass {
							subjectText = filepath.Base(ans)
						} else if pass, e := regexpMatchString(("*dir*path*|*folder*path*|*parent*path*"), statement); e == nil && pass {
							subjectText = filepath.Dir(ans)
						} else if pass, e := regexpMatchString(("*dir*name*|*folder*name*|*parent*name*"), statement); e == nil && pass {
							subjectText = filepath.Base(filepath.Dir(ans))
						} else {
							subjectText = ans
						}
						// strings.Replace(strings.Replace(strings.Replace(ans, "\\\\", "odjs,dsam2eiuh", -1), " ", "\\ ", -1), "odjs,dsam2eiuh", "\\", -1)
						//						fmt.Println(subjectText)
					}
				}

			} else if strings.Contains(statement, " post ") {

				for val, ans := range posts {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}

			} else if strings.Contains(statement, " get ") {

				for val, ans := range gets {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}

			} else if strings.Contains(statement, " request ") {

				for val, ans := range posts {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}
				if subjectText == "" {
					for val, ans := range gets {
						if paths[x] == val {
							objectText = subjectText
							subjectText = ans
						}
					}
				}

			} else {
				objectText = subjectText
				subjectText = paths[x]
			}

			if pass, e := regexpMatchString(`*js*|*eval*|*javascript*`, statement); e == nil && pass {

				val, _ := Run(globalvm[ottoNum], subjectText)

				subjectText = val.value.GetString(-1)

			}

			r, err := regexp.Compile(subjectText)
			if err == nil && r.FindString(objectText) != objectText {

				returnTrue = true
			} else {
				returnTrue = false
				//return false, x+2, doNotCache, variables
			}
		} else if pass, e := regexpMatchString(("*match*perfect*|*perfect*regex*|*regex*perfect*|*complete*regex*|*regex*complete*|*perfect*match*|*match*complete*|*complete*match*"), statement); e == nil && pass && objectText != "" {
			statementFound = true
			if pass2, _ := regexpMatchString("*global*", statement); pass2 {
				subjectText = ""
				objectText = paths[x]

				if ans, ok := glob[objectText]; ok {

					subjectText = ans
				}

			} else if pass2, _ := regexpMatchString("*arg*", statement); pass2 {

				for val, ans := range args {
					v := strconv.Itoa(val)
					if paths[x] == v {
						objectText = subjectText
						if pass, e := regexpMatchString(("*file*name*|*file*"), statement); e == nil && pass {
							subjectText = filepath.Base(ans)
						} else if pass, e := regexpMatchString(("*dir*path*|*folder*path*|*parent*path*"), statement); e == nil && pass {
							subjectText = filepath.Dir(ans)
						} else if pass, e := regexpMatchString(("*dir*name*|*folder*name*|*parent*name*"), statement); e == nil && pass {
							subjectText = filepath.Base(filepath.Dir(ans))
						} else {
							subjectText = ans
						}
						// strings.Replace(strings.Replace(strings.Replace(ans, "\\\\", "odjs,dsam2eiuh", -1), " ", "\\ ", -1), "odjs,dsam2eiuh", "\\", -1)
						//						fmt.Println(subjectText)
					}
				}

			} else if strings.Contains(statement, " post ") {

				for val, ans := range posts {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}

			} else if strings.Contains(statement, " get ") {

				for val, ans := range gets {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}

			} else if strings.Contains(statement, " request ") {

				for val, ans := range posts {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}
				if subjectText == "" {
					for val, ans := range gets {
						if paths[x] == val {
							objectText = subjectText
							subjectText = ans
						}
					}
				}

			} else {
				objectText = subjectText
				subjectText = paths[x]
			}

			if pass, e := regexpMatchString(`*js*|*eval*|*javascript*`, statement); e == nil && pass {

				val, _ := Run(globalvm[ottoNum], subjectText)

				subjectText = val.value.GetString(-1)

			}

			r, err := regexp.Compile(subjectText)
			if err == nil && r.FindString(objectText) == objectText {

				returnTrue = true
			} else {
				returnTrue = false
				//return false, x+2, doNotCache, variables
			}
		} else if pass, e := regexpMatchString(("*not*regex*|*not*match*"), statement); e == nil && pass && objectText != "" {
			//			fmt.Println(paths[x])
			statementFound = true
			if pass2, _ := regexpMatchString("*global*", statement); pass2 {

				if ans, ok := glob[paths[x]]; ok {
					objectText = ans
					subjectText = fmt.Sprint(ans)
				}

			} else if pass2, _ := regexpMatchString("*arg*", statement); pass2 {

				for val, ans := range args {
					v := strconv.Itoa(val)
					if paths[x] == v {
						objectText = subjectText
						if pass, e := regexpMatchString(("*file*name*|*file*"), statement); e == nil && pass {
							subjectText = filepath.Base(ans)
						} else if pass, e := regexpMatchString(("*dir*path*|*folder*path*|*parent*path*"), statement); e == nil && pass {
							subjectText = filepath.Dir(ans)
						} else if pass, e := regexpMatchString(("*dir*name*|*folder*name*|*parent*name*"), statement); e == nil && pass {
							subjectText = filepath.Base(filepath.Dir(ans))
						} else {
							subjectText = ans
						}
						// strings.Replace(strings.Replace(strings.Replace(ans, "\\\\", "odjs,dsam2eiuh", -1), " ", "\\ ", -1), "odjs,dsam2eiuh", "\\", -1)
						//						fmt.Println(subjectText)
					}
				}

			} else if strings.Contains(statement, " post ") {

				for val, ans := range posts {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}

			} else if strings.Contains(statement, " get ") {

				for val, ans := range gets {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}

			} else if strings.Contains(statement, " request ") {

				for val, ans := range posts {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}
				if subjectText == "" {
					for val, ans := range gets {
						if paths[x] == val {
							objectText = subjectText
							subjectText = ans
						}
					}
				}

			} else {
				objectText = subjectText
				subjectText = paths[x]
			}

			if pass, e := regexpMatchString(`*js*|*eval*|*javascript*`, statement); e == nil && pass {

				val, _ := Run(globalvm[ottoNum], subjectText)

				subjectText = val.value.GetString(-1)

			}

			if m, err := regexp.MatchString(subjectText, objectText); !m && err == nil {
				returnTrue = true
			} else {
				returnTrue = false
				//return false, x+2, doNotCache, variables
			}
		} else if pass, e := regexpMatchString(("*match*|*regex*"), statement); e == nil && pass && objectText != "" {

			statementFound = true
			if pass2, _ := regexpMatchString("*global*", statement); pass2 {

				if ans, ok := glob[paths[x]]; ok {
					objectText = ans
					subjectText = fmt.Sprint(ans)
				}

			} else if pass2, _ := regexpMatchString("*arg*", statement); pass2 {

				for val, ans := range args {
					v := strconv.Itoa(val)
					if paths[x] == v {
						objectText = subjectText
						if pass, e := regexpMatchString(("*file*name*|*file*"), statement); e == nil && pass {
							subjectText = filepath.Base(ans)
						} else if pass, e := regexpMatchString(("*dir*path*|*folder*path*|*parent*path*"), statement); e == nil && pass {
							subjectText = filepath.Dir(ans)
						} else if pass, e := regexpMatchString(("*dir*name*|*folder*name*|*parent*name*"), statement); e == nil && pass {
							subjectText = filepath.Base(filepath.Dir(ans))
						} else {
							subjectText = ans
						}
						// strings.Replace(strings.Replace(strings.Replace(ans, "\\\\", "odjs,dsam2eiuh", -1), " ", "\\ ", -1), "odjs,dsam2eiuh", "\\", -1)
						//						fmt.Println(subjectText)
					}
				}

			} else if strings.Contains(statement, " post ") {

				for val, ans := range posts {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}

			} else if strings.Contains(statement, " get ") {

				for val, ans := range gets {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}

			} else if strings.Contains(statement, " request ") {

				for val, ans := range posts {
					if paths[x] == val {
						objectText = subjectText
						subjectText = ans
					}
				}
				if subjectText == "" {
					for val, ans := range gets {
						if paths[x] == val {
							objectText = subjectText
							subjectText = ans
						}
					}
				}

			} else {
				objectText = subjectText
				subjectText = paths[x]
			}

			if pass, e := regexpMatchString(`*js*|*eval*|*javascript*`, statement); e == nil && pass {

				val, _ := Run(globalvm[ottoNum], subjectText)

				subjectText = val.value.GetString(-1)

			}

			if m, err := regexp.MatchString(subjectText, objectText); m && err == nil {
				returnTrue = true
			} else {
				returnTrue = false
				//return false, x+2, doNotCache, variables
			}
		} else if pass, e := regexpMatchString(("*to*|*with*"), statement); e == nil && pass {

			if lastVariableKey != "" {
				statementFound = true
				replaceGlobal := false

				if x >= 3 {

					if pass2, _ := regexpMatchString("*set*|*change*|*replace*", paths[x-3]); pass2 {

						if pass2, _ := regexpMatchString("*global*", paths[x-3]); pass2 {
							replaceGlobal = true

						}
					}

				}
				if pass2, _ := regexpMatchString("*global*", statement); pass2 {
					subjectText = ""
					objectText = paths[x]
					if ans, ok := glob[lastVariableKey]; ok {
						if replaceGlobal {
							glob[lastVariableKey] = ans
						} else {
							variables[lastVariableKey] = ans
						}

					}

				} else if pass2, _ := regexpMatchString("*arg*", statement); pass2 {

					for val, ans := range args {

						v := strconv.Itoa(val)

						if paths[x] == v {
							if pass, e := regexpMatchString(("*file*name*|*file*"), statement); e == nil && pass {
								variables[lastVariableKey] = filepath.Base(ans)

							} else if pass, e := regexpMatchString(("*dir*path*|*folder*path*|*parent*path*"), statement); e == nil && pass {

								if replaceGlobal {
									glob[lastVariableKey] = filepath.Dir(ans)
								} else {
									variables[lastVariableKey] = filepath.Dir(ans)
								}

							} else if pass, e := regexpMatchString(("*dir*name*|*folder*name*|*parent*name*"), statement); e == nil && pass {

								if replaceGlobal {
									glob[lastVariableKey] = filepath.Base(filepath.Dir(ans))
								} else {
									variables[lastVariableKey] = filepath.Base(filepath.Dir(ans))
								}
							} else {

								if replaceGlobal {
									glob[lastVariableKey] = ans
								} else {
									variables[lastVariableKey] = ans
								}

							}

							//strings.Replace(strings.Replace(strings.Replace(ans, "\\\\", "odjs,dsam2eiuh", -1), " ", "\\ ", -1), "odjs,dsam2eiuh", "\\", -1)
							//fmt.Println(variables[lastVariableKey])
						}
					}

				} else if strings.Contains(statement, " post ") {

					subjectText = ""
					objectText = paths[x]
					for val, ans := range posts {
						if paths[x] == val {
							if replaceGlobal {
								glob[lastVariableKey] = ans
							} else {
								variables[lastVariableKey] = ans
							}

						}
					}

				} else if strings.Contains(statement, " get ") {

					for val, ans := range gets {
						if paths[x] == val {
							if replaceGlobal {
								glob[lastVariableKey] = ans
							} else {
								variables[lastVariableKey] = ans
							}
						}
					}

				} else if strings.Contains(statement, " request ") {

					for val, ans := range posts {
						if paths[x] == val {
							if replaceGlobal {
								glob[lastVariableKey] = ans
							} else {
								variables[lastVariableKey] = ans
							}
						}
					}
					if subjectText == "" {
						for val, ans := range gets {
							if paths[x] == val {
								if replaceGlobal {
									glob[lastVariableKey] = ans
								} else {
									variables[lastVariableKey] = ans
								}
							}
						}
					}

				} else if pass, e := regexp.MatchString(convertRegex(`any\s*thing|any( (arg or )|\-)(key|arg)`), statement); e == nil && pass {

					for val, ans := range posts {
						if paths[x] == val {
							if replaceGlobal {
								glob[lastVariableKey] = ans
							} else {
								variables[lastVariableKey] = ans
							}
						}
					}
					if subjectText == "" {
						for val, ans := range gets {
							if paths[x] == val {
								if replaceGlobal {
									glob[lastVariableKey] = ans
								} else {
									variables[lastVariableKey] = ans
								}
							}
						}

						if subjectText == "" {
							for val, ans := range args {
								v := strconv.Itoa(val)
								if paths[x] == v {
									if pass, e := regexpMatchString(("*file*name*|*file*"), statement); e == nil && pass {
										if replaceGlobal {
											glob[lastVariableKey] = filepath.Base(ans)
										} else {
											variables[lastVariableKey] = filepath.Base(ans)
										}

									} else if pass, e := regexpMatchString(("*dir*path*|*folder*path*|*parent*path*"), statement); e == nil && pass {

										if replaceGlobal {
											glob[lastVariableKey] = filepath.Dir(ans)
										} else {
											variables[lastVariableKey] = filepath.Dir(ans)
										}
									} else if pass, e := regexpMatchString(("*dir*name*|*folder*name*|*parent*name*"), statement); e == nil && pass {

										if replaceGlobal {
											glob[lastVariableKey] = filepath.Base(filepath.Dir(ans))
										} else {
											variables[lastVariableKey] = filepath.Base(filepath.Dir(ans))
										}

									} else {
										if replaceGlobal {
											glob[lastVariableKey] = ans

										} else {
											variables[lastVariableKey] = ans
										}
									}

								}
							}
						}

					}

				} else {

					if replaceGlobal {
						glob[lastVariableKey] = paths[x]
					} else {
						variables[lastVariableKey] = paths[x]
					}
				}

				if pass, e := regexpMatchString(`*js*|*eval*|*javascript*`, statement); e == nil && pass {
					if replaceGlobal {
						val, _ := Run(globalvm[ottoNum], glob[lastVariableKey])
						glob[lastVariableKey] = val.value.GetString(-1)
					} else {
						val, _ := Run(globalvm[ottoNum], variables[lastVariableKey])
						variables[lastVariableKey] = val.value.GetString(-1)
					}

				}

				//				fmt.Println(paths[x], "=", lastVariableKey)
				//				fmt.Println(x, "<<<")
				lastVariableKey = ""
				if replaceGlobal {
					globalChanged = true
				}
			}

		}
		if !statementFound && returnTrue {

			if len(paths) > x {

				return true, x, doNotCache, variables
			}

		} else {

			x += 2
			endX = x

		}

	}

	return false, endX, doNotCache, variables

}

func readExecute(path string, argsString string, vmnum int, returnOutput bool, origOttoNum int, isConcurrent bool, setNoCache bool, setForceCache bool, forceLiveMode bool) string {
	tempStoretempForceDevMode := tempForceDevMode
	tempForceDevMode = forceLiveMode
	//fmt.Println(tempForceDevMode)
	tempStoretempNoCache := tempNoCache
	tempNoCache = setNoCache

	tempStoretempForceCache := tempForceCache
	tempForceCache = setForceCache
	if isConcurrent {
		defer wg.Done()
	}

	wg.Add(1)

	wTemp := new(sync.WaitGroup)
	output := ""
	//	args := strings.Fields(argsString)
	vm := globalvm[vmnum]
	if strings.Contains("\n"+path, "\nhttps://") || strings.Contains("\n"+path, "\nhttp://") {

		//wg.Add(1)

		abs := path

		//		if strings.Contains(" "+path, " asynchronous") || strings.Contains(" "+path, " async ") {

		//			//			tempDir, _ := os.Getwd()

		//			//			os.Chdir(filepath.Dir(path))

		//			//			absPath, _ := filepath.Abs(paths[x])
		//			readExecuteTODOpath = append(readExecuteTODOpath, path)
		//			readExecuteTODOargsString = append(readExecuteTODOargsString, argsString)
		//			readExecuteTODOOttoNum = append(readExecuteTODOOttoNum, origOttoNum)
		//			//			os.Chdir(tempDir)

		//		} else {

		if valString, ok := cachedScripts[fingerprint("http-file"+abs)]; ok && !tempForceDevMode && !forceCache {

			output = flushOutput(valString, returnOutput)
		} else {

			r := httpGet(path)

			scripts := strings.Split(string(r), `{{do-not-cache}}`)
			if len(scripts) > 1 {
				r = scripts[1]

				v := new(cacheVal)
				v.val = r
				output = flushOutput(v, returnOutput)

			} else {

				if valString, ok := cachedScripts[fingerprint("http-file"+abs)]; ok && !tempForceDevMode && !forceCache {
					output = flushOutput(valString, returnOutput)
				} else {

					cacheOutput("http-file"+abs, r, vmnum)

					v := new(cacheVal)
					v.val = preCachedString[origOttoNum] + r
					output = flushOutput(v, returnOutput)
					preCachedString[origOttoNum] = ""
				}
			}

			//			}
		}
	} else if strings.Contains(path+"\n", ".js\n") {

		//		wg.Add(1)
		//		if strings.Contains(" "+path, " asynchronous") || strings.Contains(" "+path, " async ") {

		//			//			tempDir, _ := os.Getwd()

		//			//			os.Chdir(filepath.Dir(path))

		//			//			absPath, _ := filepath.Abs(paths[x])
		//			readExecuteTODOpath = append(readExecuteTODOpath, path)
		//			readExecuteTODOargsString = append(readExecuteTODOargsString, argsString)
		//			readExecuteTODOOttoNum = append(readExecuteTODOOttoNum, origOttoNum)
		//			//			os.Chdir(tempDir)

		//		} else {

		abs, _ := filepath.Abs(path)
		__DIR__ = filepath.Dir(abs)
		__FILE__ = abs

		vmSetStringVar(vm, "__DIR__", __DIR__)
		vmSetStringVar(vm, "__FILE__", __FILE__)

		if valString, ok := cachedScripts[fingerprint("js-file"+abs)]; ok && !tempForceDevMode && !forceCache {
			output = flushOutput(valString, returnOutput)
		} else {
			rBytes, e := ioutil.ReadFile(path)
			r := string(rBytes)
			if e == nil {
				scripts := strings.Split(string(r), `{{do-not-cache}}`)
				if len(scripts) > 1 {
					r = scripts[1]
					val, _ := Run(vm, string(r))
					if !val.value.IsUndefined(-1) {
						v := new(cacheVal)
						v.val = preCachedString[origOttoNum] + val.value.GetString(-1)
						output = flushOutput(v, returnOutput)
						preCachedString[origOttoNum] = ""
					}

				} else {

					if valString, ok := cachedScripts[fingerprint("js-file"+abs)]; ok && !tempForceDevMode && !forceCache {
						output = flushOutput(valString, returnOutput)
					} else {

						val, _ := Run(vm, r)
						//						fmt.Println(wg.Count())
						if !val.value.IsUndefined(-1) {
							cacheOutput("js-file"+abs, val.value.GetString(-1), vmnum)

							v := new(cacheVal)
							v.val = preCachedString[origOttoNum] + val.value.GetString(-1)
							output = flushOutput(v, returnOutput)
							preCachedString[origOttoNum] = ""
						} else {
							cacheOutput("js-file"+abs, "", vmnum)

							v := new(cacheVal)
							v.val = preCachedString[origOttoNum] + val.value.GetString(-1)
							output = flushOutput(v, returnOutput)
							preCachedString[origOttoNum] = ""
						}
					}
				}
			}
			//			}
		}
	} else {

		//wg.Add(1)

		pathAndArgs := strings.Split(path, " ")

		for u := 0; u < len(pathAndArgs)-1; u++ {

			for (len(pathAndArgs[u]) >= 1 && pathAndArgs[u][len(pathAndArgs[u])-1:] == `\`) || (len(pathAndArgs[u]) >= 2 && (pathAndArgs[u][len(pathAndArgs[u])-1:] == `\` && pathAndArgs[u][len(pathAndArgs[u])-2:] != `\\`)) {
				pathAndArgs[u] = pathAndArgs[u][:len(pathAndArgs[u])-1] + ` ` + pathAndArgs[u+1]
				pathAndArgs = append(pathAndArgs[:u+1], pathAndArgs[u+2:]...)
			}
		}

		//		for len(paths[x]) < 3  && paths[x][len(paths[x])-2:] == `\"` || len(paths[x]) >= 3 && (paths[x][len(paths[x])-2:] == `\"` && paths[x][len(paths[x])-3:] != `\\"`) {
		// <<<< fix me
		//		for u := 0; u < len(pathAndArgs); u++ {
		//			for pathAndArgs[u][len(pathAndArgs[u])-1:] == "\\" && (len(pathAndArgs[u]) < 2 || pathAndArgs[u][len(pathAndArgs[u])-2:] != "\\\\") {

		//				pathAndArgs[u] = pathAndArgs[u][:len(pathAndArgs[u])-1] + " " + pathAndArgs[u+1]

		//				pathAndArgs = append(pathAndArgs[:u+1], pathAndArgs[u+2:]...)

		//			}

		//			//

		//			b := []byte(`"` + pathAndArgs[u] + `"`)
		//			s := ""
		//			err := json.Unmarshal(b, &s)

		//			if err == nil {
		//				pathAndArgs[u] = s

		//			}

		//			//pathAndArgs[u], _ = strconv.Unquote(pathAndArgs[u])

		//		}

		var cmd *exec.Cmd
		if len(pathAndArgs) > 1 {
			cmd = exec.Command(pathAndArgs[0], pathAndArgs[1:]...)
		} else {

			cmd = exec.Command(pathAndArgs[0])
		}

		//cmd.Stdin = strings.NewReader("")
		mode := ""

		info, err := os.Stat(pathAndArgs[0])
		if err == nil {
			mode = fmt.Sprint(info.Mode())
		}

		var out bytes.Buffer

		if strings.Contains(mode, "x") {

			// run the file if it is marked as an executable
			cmd.Stdout = &out
			err = cmd.Run()
		}

		if (strings.Contains(mode, "x") && err == nil) || len(out.Bytes()) > 0 {

			// if their are no errors or if an output was received continue
			v := new(cacheVal)
			v.val = preCachedString[origOttoNum] + out.String()
			output = flushOutput(v, returnOutput)
			preCachedString[origOttoNum] = ""
		} else if (strings.Contains(path+"\n", ".template\n")) || (strings.Contains(path+"\n", ".routine\n") && isConcurrent) {

			//wg.Add(1)
			rBytes, _ := ioutil.ReadFile(path)
			r := string(rBytes)
			r = " " + (r)
			paths := SplitStatementsFromInputs(r)

			Allvariables := make(map[string]string)
			for x := 1; x < len(paths); {

				//fmt.Println(Allvariables)

				doContinue, newX, _, Allvariables := testRequest(x, paths, os.Args, map[string]string{}, map[string]string{}, noCache, Allvariables, origOttoNum)

				for varkey, varval := range Allvariables {

					paths[x] = strings.Replace(paths[x], varkey, varval, -1)
				}

				x = newX
				if !doContinue {
					//				wg.Done()
					continue
				} else {

				}

				//			fmt.Println(len(paths), x)

				if strings.Contains(paths[x]+"\n", ".routine\n") || strings.Contains(" "+paths[x-1], " asynchronous") || strings.Contains(" "+paths[x-1], " async ") || strings.Contains(paths[x-1], "asynchronous") || strings.Contains(paths[x-1], "\nasync ") || strings.Contains(paths[x-1], "\nasynchronous") {
					routinesTriggered = true
					tempDir, _ := os.Getwd()

					os.Chdir(filepath.Dir(path))
					absPath, _ := filepath.Abs(paths[x])

					todoNoCache, _ := testIfShouldNOTCache(paths[x-1])
					if _, err := os.Stat(absPath); !os.IsNotExist(err) {
						wg.Add(1)
						readExecuteTOdoLiveMode = append(readExecuteTOdoLiveMode, testIfShouldLiveUpdate(paths[x-1]))
						readExecuteTOdoNotCache = append(readExecuteTOdoNotCache, todoNoCache)
						readExecuteTODOpath = append(readExecuteTODOpath, absPath)
						readExecuteTODOargsString = append(readExecuteTODOargsString, argsString)
						readExecuteTODOOttoNum = append(readExecuteTODOOttoNum, origOttoNum)

					} else if !strings.Contains(paths[x]+"\n", ".js\n") && (strings.Contains("\n"+paths[x], "http://") || strings.Contains("\n"+paths[x], "https://")) {
						wg.Add(1)
						readExecuteTOdoLiveMode = append(readExecuteTOdoLiveMode, testIfShouldLiveUpdate(paths[x-1]))
						readExecuteTOdoNotCache = append(readExecuteTOdoNotCache, todoNoCache)
						readExecuteTODOpath = append(readExecuteTODOpath, paths[x])
						readExecuteTODOargsString = append(readExecuteTODOargsString, argsString)
						readExecuteTODOOttoNum = append(readExecuteTODOOttoNum, origOttoNum)
					} else if strings.Contains(paths[x]+"\n", ".js\n") {
						wg.Add(1)
						readExecuteTOdoLiveMode = append(readExecuteTOdoLiveMode, testIfShouldLiveUpdate(paths[x-1]))
						readExecuteTOdoNotCache = append(readExecuteTOdoNotCache, todoNoCache)
						readExecuteTODOpath = append(readExecuteTODOpath, paths[x])
						readExecuteTODOargsString = append(readExecuteTODOargsString, argsString)
						readExecuteTODOOttoNum = append(readExecuteTODOOttoNum, origOttoNum)
					}

					os.Chdir(tempDir)

				} else {
					//wg.Add(1)
					cwd, _ := os.Getwd()

					absPath, _ := filepath.Abs(path)
					os.Chdir(filepath.Dir(absPath))
					todoNoCache, forceCache := testIfShouldNOTCache(paths[x-1])
					tempForceCache = tempStoretempForceCache
					tempForceDevMode = tempStoretempForceDevMode
					tempNoCache = tempStoretempNoCache
					readExecute(paths[x], argsString, vmnum, returnOutput, origOttoNum, isConcurrent, todoNoCache, forceCache, testIfShouldLiveUpdate(paths[x-1]))
					tempStoretempNoCache = tempNoCache
					tempStoretempForceDevMode = tempForceDevMode
					tempStoretempForceCache = tempForceCache
					os.Chdir(cwd)

				}

				x += 2
			}
		} else if strings.Contains(filepath.Base(pathAndArgs[0]), ".") {

			abs, _ := filepath.Abs(pathAndArgs[0])

			if valString, ok := cachedScripts[fingerprint("txt-file"+abs)]; ok && !tempForceDevMode && !forceCache {
				output = flushOutput(valString, returnOutput)

			} else {

				rBytes, e := ioutil.ReadFile(abs)

				if e == nil {
					r := string(rBytes)
					tempCacheOutput := noscriptargs
					noscriptargs = []string{}

					cacheOutput("txt-file"+abs, r, vmnum)

					noscriptargs = tempCacheOutput
					// replace noscriptargs with empty []string because it does not matter what args are passed, this just duplicates the text file.
					// once it has been cached, this gets replaced
					v := new(cacheVal)
					v.val = preCachedString[origOttoNum] + r
					output = flushOutput(v, returnOutput)
					preCachedString[origOttoNum] = ""

				}

			}
			//		}
		}
	}

	wTemp.Wait()
	defer wg.Done()
	tempStoretempNoCache = tempNoCache
	tempStoretempForceDevMode = tempForceDevMode
	tempStoretempForceCache = tempForceCache
	return output
}

func killScript() {
	os.Exit(1)
}

func flushOutput(out *cacheVal, returnOutput bool) string {

	out.calls = out.calls + callIncrement
	if returnOutput {
		return out.val
	}
	fmt.Print(out.val)

	return ""
}

var sizeOfCache float64 = 0.0
var countCacheExecutes float64 = 0.0

func cacheOutput(in, out string, vmNum int) {

	if tempNoCache {

		return
	}

	out = preCachedString[vmNum] + out // output the precache and the fresh cache at the same time

	if !tempForceDevMode || tempForceCache {

		if sizeOfCache > 1024.0*1024.0*cacheMBSize {
			// implement cache control
			purgeCache()
		}

		countCacheExecutes = countCacheExecutes + 1

		f := fingerprint(in)
		v := new(cacheVal)
		if _, ok := cachedScripts[f]; !ok {

			v.val = out
			v.calls = 0
			cachedScripts[f] = v
			sizeOfCache = sizeOfCache + float64(len([]byte(out)))

			cacheChanged = true

		} else {
			if cachedScripts[f].val != out {
				sizeOfCache = sizeOfCache - float64(len([]byte(cachedScripts[f].val)))
				v.val = out
				v.calls = 0
				cachedScripts[f] = v
				sizeOfCache = sizeOfCache + float64(len([]byte(out)))

				cacheChanged = true
			}
		}

	}
}

func purgeCache() {
	if sizeOfCache > 1024.0*1024.0*cacheMBSize {
		minCalls := 99999999999.0
		maxCalls := -99999999999.0
		calls := maxCalls - 0.1
		for calls <= maxCalls {

			for k, v := range cachedScripts {

				if v.calls > maxCalls {
					maxCalls = v.calls
				}

				if v.calls < minCalls {
					minCalls = v.calls
				}

				if v.calls <= calls {
					// delete random cached outputs until cache size is at an acceptabel limit
					sizeOfCache = sizeOfCache - float64(len([]byte(v.val)))
					delete(cachedScripts, k)

					if sizeOfCache > 1024.0*1024.0*cacheMBSize {

					} else {

						return
					}
				}
			}

			if calls < minCalls {
				calls = minCalls
			} else {
				calls = calls + callIncrement
			}

			if sizeOfCache > 1024.0*1024.0*cacheMBSize {

			} else {

				return
			}

		}
	}
}

func fingerprint(template string) string {

	hasher := md5.New()
	wd, _ := os.Getwd()
	hasher.Write([]byte(wd + " - " + strings.Join(noscriptargs, " ") + template))
	return hex.EncodeToString(hasher.Sum(nil))

}

func loadOtto(num int) {

	vm := globalvm[num]

	vm.PushGlobalGoFunction("set_timeout", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			maxExecutionTime = time.Millisecond * time.Duration(c.GetNumber(0))
		}
		return 0
	})
	//	.Set("set_time_limit", func(milliseconds int64) {
	//		maxExecutionTime = time.Millisecond * time.Duration(milliseconds)
	//	})

	vm.PushGlobalGoFunction("flush", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			output := c.GetString(0)
			v := new(cacheVal)
			v.val = output
			preCachedString[num] = preCachedString[num] + output
			flushOutput(v, false)
			preCachedString[num] = ""
		}

		return 0
	})
	vm.PushGlobalGoFunction("file_exists", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			path := c.GetString(0)
			tempDir, _ := os.Getwd()
			os.Chdir(__DIR__)
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				// path/to/whatever exists
				os.Chdir(tempDir)
				c.PushBoolean(true)
				return 1
			}
			os.Chdir(tempDir)
			return 0
		}
		return 0
	})
	vm.PushGlobalGoFunction("require", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			path := c.GetString(0)
			//	vm.Set("require", func(path string) string {
			tempDir, _ := os.Getwd()

			temp__DIR__ := __DIR__
			temp__FILE__ := __FILE__

			os.Chdir(__DIR__)
			out := readExecute(path, strings.Join(noscriptargs, " "), num, true, num, false, tempNoCache, tempForceCache, tempForceDevMode)
			os.Chdir(tempDir)
			__FILE__ = temp__FILE__
			__DIR__ = temp__DIR__

			vmSetStringVar(vm, "__DIR__", __DIR__)
			vmSetStringVar(vm, "__FILE__", __FILE__)
			c.PushString(out)
			return 1
		}

		return 0
	})
	vm.PushGlobalGoFunction("CacheSize", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			cacheMBSize = c.GetNumber(0)
		}
		return 0
	})
	//	vm.Set("surf", sf)
	vm.PushGlobalGoFunction("ReadFile", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			path := c.GetString(0)
			tempDir, _ := os.Getwd()

			temp__DIR__ := __DIR__
			temp__FILE__ := __FILE__

			os.Chdir(__DIR__)

			b, _ := ioutil.ReadFile(path)

			os.Chdir(tempDir)
			__FILE__ = temp__FILE__
			__DIR__ = temp__DIR__

			vmSetStringVar(vm, "__DIR__", __DIR__)
			vmSetStringVar(vm, "__FILE__", __FILE__)
			c.PushString(string(b))
			return 1

		}

		return 0
	})
	vm.PushGlobalGoFunction("ReadTextFile", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			path := c.GetString(0)

			tempDir, _ := os.Getwd()

			temp__DIR__ := __DIR__
			temp__FILE__ := __FILE__

			os.Chdir(__DIR__)

			b, _ := ioutil.ReadFile(path)

			os.Chdir(tempDir)
			__FILE__ = temp__FILE__
			__DIR__ = temp__DIR__

			vmSetStringVar(vm, "__DIR__", __DIR__)
			vmSetStringVar(vm, "__FILE__", __FILE__)
			c.PushString(string(b))
			return 1
		}
		return 0
	})

	//	bts := makeObject()
	//	bts["NewBuffer"] = bytes.NewBuffer

	//	vmSet("bytes", bts)

	vm.PushGlobalGoFunction("Sleep", func(c *duktape.Context) int {
		if c.GetTop() > 0 {

			time.Sleep(time.Millisecond * time.Duration(c.GetNumber(0)))
		}

		return 0
	})
	vm.PushGlobalGoFunction("ReadGlobal", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			key := c.GetString(0)
			mutex.RLock()
			ok := false
			for !ok {
				if _, ok = glob[key]; !ok {
					time.Sleep(time.Microsecond)
				}
			}
			ans := glob[key]
			mutex.RUnlock()
			c.PushString(ans)
			return 1
		}
		return 0
	})
	vm.PushGlobalGoFunction("ReadGlobalOnChange", func(c *duktape.Context) int {
		if c.GetTop() > 1 {
			key := c.GetString(0)
			timeout := int(c.GetNumber(1))

			mutex.RLock()
			ok := false
			timeoutDone := false

			go func() {
				time.Sleep(time.Duration(timeout) * time.Millisecond)
				mutex.RUnlock()
				mutex.Lock()
				timeoutDone = true
				mutex.Unlock()
				mutex.RLock()
			}()

			if _, ok = glob[key]; !ok {
				time.Sleep(time.Microsecond)
			}

			var ans string

			if interf, ok := glob[key]; ok {
				ans = interf

				ans = glob[key]
				for ans != glob[key] && !timeoutDone {
					time.Sleep(time.Millisecond)
				}

			} else {
				ok := false
				for !ok {
					if _, ok = glob[key]; !ok {

						time.Sleep(time.Millisecond)

					}

					if timeoutDone {
						ok = true
					}
				}
			}

			if ans, ok := glob[key]; ok {
				ans = ans
			}

			mutex.RUnlock()

			c.PushString(ans)
			return 1
		}

		return 0
	})
	vm.PushGlobalGoFunction("DeleteGlobal", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			key := c.GetString(0)

			//if !tempForceDevMode {
			globalChanged = true
			//}

			mutex.Lock()
			delete(glob, key)
			mutex.Unlock()
		}
		return 0
	})
	vm.PushGlobalGoFunction("WriteGlobal", func(c *duktape.Context) int {
		if c.GetTop() > 1 {
			key := c.GetString(0)
			data := c.GetString(1)

			//if !tempForceDevMode {
			globalChanged = true
			//}

			mutex.Lock()

			glob[key] = data

			mutex.Unlock()

		}
		return 0

	})

	vm.PushGlobalGoFunction("DownloadFile", func(c *duktape.Context) int {
		if c.GetTop() > 1 {
			url := c.GetString(0)
			path := c.GetString(1)

			// don't worry about errors
			response, e := http.Get(url)
			if e != nil {
				//fmt.Println(e)
				c.PushBoolean(false)
				return 1
			}
			defer response.Body.Close()

			//open a file for writing
			file, err := os.Create(path)
			if err != nil {
				//fmt.Println(err)
				c.PushBoolean(false)
				return 1
			}
			defer file.Close()

			// Use io.Copy to just dump the response body to the file. This supports huge files
			_, err = io.Copy(file, response.Body)
			if err != nil {
				//fmt.Println(err)
				c.PushBoolean(false)
				return 1
			}
			c.PushBoolean(true)
			return 1

		}

		return 0
	})
	vmSetStringVar(vm, "__DIR__", __DIR__)
	vmSetStringVar(vm, "__FILE__", __FILE__)
	vm.PushGlobalGoFunction("ListFiles", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			path := c.GetString(0)

			s := []string{}
			files, err := ioutil.ReadDir(path)

			if err != nil {
				//fmt.Println(err)
			} else {

				for _, f := range files {

					if err != nil {
						//fmt.Println(f.Name(), "<br>")
					} else {

						if f.IsDir() {
							// do directory stuff
							//fmt.Println("directory")

						} else {
							// do file stuff
							//fmt.Println("file")
							abs, _ := filepath.Abs(path + string(os.PathSeparator) + f.Name())
							s = append(s, abs)
						}

					}
				}
			}

			b, _ := json.Marshal(s)

			c.PevalString(`JSON.parse("` + strings.Replace(string(b), `"`, `\"`, -1) + `");`)

			return 1

		}

		return 0
	})

	vm.PushGlobalGoFunction("ListDirectories", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			path := c.GetString(0)

			s := []string{}
			files, err := ioutil.ReadDir(path)

			if err != nil {
				//fmt.Println(err)
			} else {

				for _, f := range files {

					if err != nil {

					} else {
						if !f.IsDir() {
							// do directory stuff
							//fmt.Println("directory")

						} else {
							// do file stuff
							//fmt.Println("file")
							abs, _ := filepath.Abs(path + string(os.PathSeparator) + f.Name())
							s = append(s, abs)
						}
					}

				}
			}

			b, _ := json.Marshal(s)

			c.PevalString(`JSON.parse("` + strings.Replace(string(b), `"`, `\"`, -1) + `");`)

			return 1

		}

		return 0
	})

	//	vm.Set("WriteImageFile", func(path string, image *browser.Image) string {

	//		tempDir, _ := os.Getwd()

	//		temp__DIR__ := __DIR__
	//		temp__FILE__ := __FILE__

	//		os.Chdir(__DIR__)

	//		fout, err := os.Create(path)
	//		if err != nil {

	//			os.Chdir(tempDir)
	//			__FILE__ = temp__FILE__
	//			__DIR__ = temp__DIR__

	//			vm.Set("__DIR__", __DIR__)
	//			vm.Set("__FILE__", __FILE__)

	//			return fmt.Sprint(
	//				"Error creating file '%s'.", path)

	//		}
	//		defer fout.Close()

	//		_, err = image.Download(fout)
	//		if err != nil {

	//			os.Chdir(tempDir)
	//			__FILE__ = temp__FILE__
	//			__DIR__ = temp__DIR__

	//			vm.Set("__DIR__", __DIR__)
	//			vm.Set("__FILE__", __FILE__)

	//			return fmt.Sprint(
	//				"Error downloading file '%s'.", path)
	//		}

	//		os.Chdir(tempDir)
	//		__FILE__ = temp__FILE__
	//		__DIR__ = temp__DIR__

	//		vm.Set("__DIR__", __DIR__)
	//		vm.Set("__FILE__", __FILE__)

	//		return ""

	//	})

	vm.PushGlobalGoFunction("WriteTextFile", func(c *duktape.Context) int {
		if c.GetTop() > 2 {
			path := c.GetString(0)
			str := c.GetString(1)
			perm := uint32(c.GetNumber(2))

			tempDir, _ := os.Getwd()

			temp__DIR__ := __DIR__
			temp__FILE__ := __FILE__

			os.Chdir(__DIR__)

			err := fmt.Sprint(ioutil.WriteFile(path, []byte(str), os.FileMode(perm)))

			os.Chdir(tempDir)
			__FILE__ = temp__FILE__
			__DIR__ = temp__DIR__

			vmSetStringVar(vm, "__DIR__", __DIR__)
			vmSetStringVar(vm, "__FILE__", __FILE__)
			c.PushString(fmt.Sprint(err))
			return 1
		}
		return 0
	})

	vm.PushGlobalGoFunction("DeleteFile", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			path := c.GetString(0)
			c.PushString(fmt.Sprint(os.Remove(path)))
			return 1
		}
		return 0
	})

	vm.PushGlobalGoFunction("DeleteFolder", func(c *duktape.Context) int {
		if c.GetTop() > 0 {
			path := c.GetString(0)
			c.PushString(fmt.Sprint(os.RemoveAll(path)))
			return 1
		}
		return 0
	})

	//	vm.Set("s2b", func(s string) []byte {
	//		return []byte(s)
	//	})
	//vm.PushGlobalGoFunction("b2s", func(c *duktape.Context) int {
	//		if c.GetTop() > 0 {
	//			path := c.GetString(0)
	//	vm.Set("b2s", func(b []byte) string {

	//		return fmt.Sprint(b)
	//	})

	//	u := makeObject()

	//	u["Parse"] = url.Parse

	//	vm.Set("url", u)

	vm.PevalString(`var url = new Function('return this;')();`)
	vm.PushGlobalGoFunction("URLResolveReference", func(c *duktape.Context) int {
		if c.GetTop() > 1 {
			base := c.GetString(0)
			ref := c.GetString(1)
			u, _ := url.Parse(base)
			u2, _ := url.Parse(ref)
			c.PushString(u.ResolveReference(u2).String())
			return 1
		}
		return 0
	})

	vm.PevalString(`url['resolveReference']  = URLResolveReference;`)
	//	o := makeObject()
	vm.PevalString(`var os = new Function('return this;')();`)
	b, _ := json.Marshal(os.Args)
	vm.PevalString(`os['Args'] = ` + `JSON.parse("` + strings.Replace(string(b), `"`, `\"`, -1) + `");`)
	b, _ = json.Marshal(os.PathSeparator)
	vm.PevalString(`os['PathSeparator'] = ` + `JSON.parse("` + strings.Replace(string(b), `"`, `\"`, -1) + `");`)
	b, _ = json.Marshal(os.PathListSeparator)
	vm.PevalString(`os['PathListSeparator'] = ` + `JSON.parse("` + strings.Replace(string(b), `"`, `\"`, -1) + `");`)
	b, _ = json.Marshal(noscriptargs)
	vm.PevalString(`os['Params'] = ` + `JSON.parse("` + strings.Replace(string(b), `"`, `\"`, -1) + `");`)

	//	vm.PevalString(`goquery = ` + `new Function('return this;')();`)

	//	gq["NewDocumentFromString"] = func(s string) *goquery.Document {
	//		html := bytes.NewBufferString(s)
	//		d, _ := goquery.NewDocumentFromReader(html)

	//		return d
	//	}

	//	gq["Selection"] = new(goquery.Selection)
	//	gq["NodeName"] = goquery.NodeName
	//	vm.Set("goquery", gq)

	//	vm.Set("panic", func(err error) {
	//		panic(err)
	//	})

	util, e := ioutil.ReadFile(filepath.Dir(os.Args[0]) + "/" + "utils.js")
	if e != nil {
		//fmt.Println(e)
	}

	Run(vm, string(util))

}

func makeObject() map[string]interface{} {
	return make(map[string]interface{})
}

type SafeWaitGroup struct {
	count      int
	tempWgUsed bool
	tempWg     *sync.WaitGroup
	WgUsed     int
	Wg         []*sync.WaitGroup

	waiting bool
}

func (wg *SafeWaitGroup) Count() int {
	return wg.count
}

func (wg *SafeWaitGroup) Done() {

	if wg.count > 0 && wg.waiting {
		if len(wg.Wg) > 0 {
			wg.Wg[len(wg.Wg)-1].Done()
		}

	}
	wg.count--

}
func (wg *SafeWaitGroup) Add(i int) {
	wg.count += i
	if wg.count > 0 {
		if !wg.waiting {

			//		wg.Wg.Add(i)
		} else {
			if len(wg.Wg) > 0 {
				wg.Wg[len(wg.Wg)-1].Add(i)
			}

		}
	}

}
func (wg *SafeWaitGroup) Wait() {

	//	if !wg.waiting && wg.WgUsed == 0 {

	//		wg.WgUsed = 1
	//		wg.Wg = new(sync.WaitGroup)
	//	} else if !wg.waiting && wg.WgUsed == 1 {

	//		wg.WgUsed = 2
	//		wg.Wg2 = new(sync.WaitGroup)
	//	} else if !wg.waiting && wg.WgUsed == 2 {

	//		wg.WgUsed = 0
	//		wg.Wg = new(sync.WaitGroup)
	//	}

	if wg.count > 0 {

		wg.waiting = true
		if len(wg.Wg) > 1 {
			wg.Wg = wg.Wg[1:]
		}
		wg.Wg = append(wg.Wg, new(sync.WaitGroup))
		wg.Wg[len(wg.Wg)-1].Add(wg.count)
		wg.Wg[len(wg.Wg)-1].Wait() // <<<<<<<

	}

	wg.waiting = false

}

func (wg *SafeWaitGroup) KeepWaiting() {
	wg.Wait()

	for wg.count > 0 {
		wg.Wait()

	}
}

var wg *SafeWaitGroup = new(SafeWaitGroup)

var glob map[string]string
var nomorescripts bool = false
var noscriptargs []string = []string{}
var w sync.WaitGroup
var gow sync.WaitGroup
var globalArgs []string = []string{}

func main() {

	glob = make(map[string]string)

	if len(os.Args) < 2 {
		os.Exit(1)
	}

	maxExecutionTime = time.Second * 60

	startTime := time.Now()

	w.Add(2)

	go func() {
		defer w.Done()
		b, e := ioutil.ReadFile(filepath.Dir(os.Args[0]) + "/" + "globals.gob")
		if e == nil {
			decodeGlobal(b)
		}

	}()
	go func() {

		defer w.Done()
		b, e := ioutil.ReadFile(filepath.Dir(os.Args[0]) + "/" + "cache.gob")

		if e == nil {
			decodeCache(b)

			for _, data := range cachedScripts {
				sizeOfCache = sizeOfCache + float64(len([]byte(data.val)))
			}

			purgeCache()

		}
	}()

	w.Wait()

	//wg.Add(len(os.Args) - 1)

	var blankJSScripts int = 1

	for k := 1; k < len(os.Args); k++ {
		if os.Args[k] == "-force-cache" || os.Args[k] == "-cache" {
			//wg.Done()
			blankJSScripts++
			forceCache = true
		} else if os.Args[k] == "-no-cache" {
			//wg.Done()
			blankJSScripts++
			noCache = true
		} else if forceCache == false && (os.Args[k] == "-dev" || os.Args[k] == "-development") {
			//wg.Done()
			blankJSScripts++
			isdevmode = true
			tempForceDevMode = isdevmode
		} else {

			if os.Args[k] == "-" {
				// to stop app from looking for more scripts so I can use the command line args in a different way, just put a "-" as an arg and every arg after that will not be ignored unless used in the script.
				nomorescripts = true
			} else if nomorescripts {
				noscriptargs = append(noscriptargs, os.Args[k])
				globalArgs = os.Args[k+1:]
			}

			if nomorescripts {
				//wg.Done()
			} else {
				globalvm = append(globalvm, duktape.New())

				go func(arg string, k int) {

					loadOtto(k)

					// on a new thread remove the pre-cache..
					preCachedString[k] = ""

					readExecute(arg, "", k, false, k, false, noCache, forceCache, isdevmode)

					//					if !routinesTriggered {
					//						wg.Done()
					//					}
					//time.Sleep(time.Microsecond * 1000)
					defer wg.Done()
				}(os.Args[k], k-blankJSScripts)

			}
		}

	}

	//gow.Wait()
	go func() {

		for wg.Count() > 0 {

			if time.Since(startTime) >= maxExecutionTime {
				panic("")
			}

			if len(readExecuteTODOpath) > 0 && len(readExecuteTODOOttoNum) > 0 && len(readExecuteTODOargsString) > 0 && len(readExecuteTOdoNotCache) > 0 && len(readExecuteTOdoLiveMode) > 0 {
				//				fmt.Println(wg.Count(), len(readExecuteTODOpath))

				mutex.RLock()
				num := 0
				path := ""
				argsString := ""

				Cache := readExecuteTOdoNotCache[0]
				liveUpdate := readExecuteTOdoLiveMode[0]

				tempCacheDecision := Cache
				tempForceDevMode := liveUpdate
				num = readExecuteTODOOttoNum[0]
				path = readExecuteTODOpath[0]
				argsString = readExecuteTODOargsString[0]

				cacheArr := readExecuteTOdoNotCache[1:]
				liveArr := readExecuteTOdoLiveMode[1:]
				pathArr := readExecuteTODOpath[1:]
				argsStringArr := readExecuteTODOargsString[1:]
				OttoNumArr := readExecuteTODOOttoNum[1:]

				mutex.RUnlock()
				mutex.Lock()
				readExecuteTOdoLiveMode = liveArr
				readExecuteTOdoNotCache = cacheArr
				readExecuteTODOpath = pathArr
				readExecuteTODOargsString = argsStringArr
				readExecuteTODOOttoNum = OttoNumArr
				preCachedString = append(preCachedString, "")
				mutex.Unlock()

				k := num

				if !strings.Contains(path+"\n", ".js\n") && (strings.Contains("\n"+path, "http://") || strings.Contains("\n"+path, "https://")) {

					go readExecute(path, argsString, num, false, num, true, tempCacheDecision, !tempCacheDecision, tempForceDevMode)

				} else if strings.Contains(path+"\n", ".js\n") {
					globalvm = append(globalvm, duktape.New())
					k = len(globalvm) - 1
					loadOtto(k)
					go readExecute(path, argsString, k, false, num, true, tempCacheDecision, !tempCacheDecision, tempForceDevMode)
				} else {
					globalvm = append(globalvm, duktape.New())
					k = len(globalvm) - 1
					loadOtto(k)
					go readExecute(path, argsString, k, false, num, true, tempCacheDecision, !tempCacheDecision, tempForceDevMode)
				}

			}

		}

	}()
	wg.Add(1)

	wg.KeepWaiting()

	//		wg.Wait()

	if globalChanged {
		ioutil.WriteFile(filepath.Dir(os.Args[0])+"/"+"globals.gob", encodeGlobal(), 0777)
	}
	if cacheChanged {
		ioutil.WriteFile(filepath.Dir(os.Args[0])+"/"+"cache.gob", encodeCache(), 0777)
	}

	time.Sleep(time.Millisecond)

}
