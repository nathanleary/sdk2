// language-utils
package main

import (
	"encoding/json"
	"strings"
)

func regexpMatchString(psudoRegexCode, statement string) (bool, error) {

	if statement == "" {
		return false, nil
	}

	psudoRegexArray := strings.Split(psudoRegexCode, "|")
	statement = strings.ToLower(statement)
	for y := 0; y < len(psudoRegexArray); y++ {
		RegCards := []string{}
		for len(psudoRegexArray[y]) > 0 {
			if strings.Index(psudoRegexArray[y], `\\`) == 0 {
				RegCards = append(RegCards, `\\`)
				psudoRegexArray[y] = psudoRegexArray[y][2:]
			} else if strings.Index(psudoRegexArray[y], "\\?") == 0 {
				RegCards = append(RegCards, "\\?")
				psudoRegexArray[y] = psudoRegexArray[y][2:]
			} else if strings.Index(psudoRegexArray[y], "?") == 0 {
				RegCards = append(RegCards, "?")
				psudoRegexArray[y] = psudoRegexArray[y][1:]
			} else if strings.Index(psudoRegexArray[y], "\\*") == 0 {
				RegCards = append(RegCards, "\\*")
				psudoRegexArray[y] = psudoRegexArray[y][2:]
			} else if strings.Index(psudoRegexArray[y], "*") == 0 {
				RegCards = append(RegCards, "*")
				psudoRegexArray[y] = psudoRegexArray[y][1:]
			} else if strings.Index(psudoRegexArray[y], `\s`) == 0 {
				RegCards = append(RegCards, `\s`)
				psudoRegexArray[y] = psudoRegexArray[y][2:]
			} else {
				RegCards = append(RegCards, psudoRegexArray[y][:1])
				psudoRegexArray[y] = psudoRegexArray[y][1:]
			}

		}

		stringInMemory := ""
		failedTest := false
		tempStatement := statement
		for z := 0; z < len(RegCards); z++ {

			if (tempStatement != "" || stringInMemory != "") && !failedTest {
				if RegCards[z] == "?" {

					stringInMemory = stringInMemory + tempStatement[:1]
					tempStatement = tempStatement[1:]

					if len(stringInMemory) <= 1 && z+1 == len(RegCards) {
						//if their is one char OR nothing in the string AND if the last card is a single wildcard then pass the test
						return true, nil
					}

				} else if RegCards[z] == "*" {
					//for len(tempStatement) > 0 {
					stringInMemory = stringInMemory + tempStatement
					tempStatement = ""

					//}

					if z+1 == len(RegCards) {
						//if their is one char OR nothing in the string OR if the last card is a wildcard then pass the test
						//fmt.Println(statement, psudoRegexCode)
						return true, nil
					}

				} else if RegCards[z] == `\s` {
					p := true
					for p {
						if len(stringInMemory) == 0 {
							if tempStatement[:1] != " " && tempStatement[:1] != "\t" && tempStatement[:1] != "\r" && tempStatement[:1] != "\n" {
								p = false
							} else {
								tempStatement = tempStatement[1:]
							}
						} else {
							tempStatement = stringInMemory + tempStatement
							spacePos := -1
							if strings.Contains(tempStatement, " ") {
								ps := strings.Index(tempStatement, " ")
								if spacePos < ps {
									spacePos = ps
								}

							}

							if strings.Contains(tempStatement, "\t") {
								ps := strings.Index(tempStatement, "\t")
								if spacePos < ps {
									spacePos = ps
								}
							}

							if strings.Contains(tempStatement, "\n") {
								ps := strings.Index(tempStatement, "\n")
								if spacePos < ps {
									spacePos = ps
								}
							}

							if strings.Contains(tempStatement, "\r") {
								ps := strings.Index(tempStatement, "\r")
								if spacePos < ps {
									spacePos = ps
								}
							}

							if spacePos == -1 {
								p = false
							} else {
								tempStatement = tempStatement[spacePos+1:]
								stringInMemory = ""
							}
						}
					}
				} else if len(stringInMemory) > 0 {
					specialCard := ""
					for t := z; t < len(RegCards); t++ {
						if RegCards[t] == "*" || RegCards[t] == "?" {
							break
						} else if RegCards[t] == `\\` {
							specialCard = specialCard + `\`
						} else if RegCards[t] == `\?` {
							specialCard = specialCard + `?`
						} else if RegCards[t] == `\*` {
							specialCard = specialCard + `*`
						} else {
							specialCard = specialCard + RegCards[t]
						}
					}

					if strings.Contains(stringInMemory+tempStatement, specialCard) {

						pos := strings.Index(stringInMemory, specialCard)

						if pos >= 0 {
							// if it is in the wildcard string then use that
							tempStatement = stringInMemory[pos+1:] + tempStatement
						} else {
							//else drop the wildcard  string and find the character
							pos := strings.Index(tempStatement, specialCard)
							tempStatement = tempStatement[pos+1:]
						}
						stringInMemory = ""

					} else {

						failedTest = true
						break
					}

				} else if stringInMemory == "" {
					specialCard := ""
					for t := z; t < len(RegCards); t++ {
						if RegCards[t] == "*" || RegCards[t] == "?" {
							break
						} else if RegCards[t] == `\\` {
							specialCard = specialCard + `\`
						} else if RegCards[t] == `\?` {
							specialCard = specialCard + `?`
						} else if RegCards[t] == `\*` {
							specialCard = specialCard + `*`
						} else {
							specialCard = specialCard + RegCards[t]
						}
					}
					if strings.Contains(stringInMemory+tempStatement, specialCard) {

						pos := strings.Index(stringInMemory, specialCard)

						if pos >= 0 {
							// if it is in the wildcard string then use that
							tempStatement = stringInMemory[pos+1:] + tempStatement
						} else {
							//else drop the wildcard  string and find the character
							pos := strings.Index(tempStatement, specialCard)
							tempStatement = tempStatement[pos+1:]
						}
						stringInMemory = ""
					} else {
						failedTest = true
						break
					}

				} else {

					failedTest = true
					break
				}
			} else if !failedTest {
				if tempStatement == "" && (z+1 >= len(RegCards) || strings.Replace(strings.Join(RegCards[z+1:], ""), "", "*", -1) == "") {

					return true, nil
				} else {

					failedTest = true

					break
				}
				// still cards to go, does not pass test
			} else {

				failedTest = true
				break
			}

			if tempStatement == "" && stringInMemory == "" && (z+1 >= len(RegCards) || strings.Replace(strings.Join(RegCards[z+1:], ""), "", "*", -1) == "") && !failedTest {
				//fmt.Println(statement, psudoRegexCode)
				return true, nil
			} else if tempStatement == "" && stringInMemory == "" && (z+1 >= len(RegCards) || strings.Replace(strings.Join(RegCards[z+1:], ""), "", "*", -1) == "") {

				failedTest = true

				break
			}

		}

	}

	return false, nil

}

func SplitStatementsFromInputs(path string) []string {

	paths := strings.Split(path, "`")

	for x := 0; x < len(paths)-1; x += 1 {

		for (len(paths[x]) >= 2 && paths[x][len(paths[x])-2:] == "\\`") || (len(paths[x]) >= 3 && (paths[x][len(paths[x])-2:] == "\\`" && paths[x][len(paths[x])-3:] != "\\\\`")) {

			paths[x] = paths[x][:len(paths[x])-1] + "`" + paths[x+1]

			paths = append(paths[:x+1], paths[x+2:]...)

		}

		//paths[x] = strings.Replace(strings.Replace(paths[x], `\ `, ` `, -1), `\ `, `\ `, -1)

		f1 := paths[x]

		b := []byte(`"` + f1 + `"`)
		s := ""
		err := json.Unmarshal(b, &s)
		if err == nil {
			paths[x] = strings.Replace(paths[x], f1, s, 1)
		}

	}

	return paths
}
