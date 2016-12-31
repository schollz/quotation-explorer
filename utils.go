package main

import (
	"log"
	"strings"
	"time"
)

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}

const delim = "?!.;,*"

func isDelim(c string) bool {
	if strings.Contains(delim, c) {
		return true
	}
	return false
}

func cleanString(input string) string {
	size := len(input)
	temp := ""
	var prevChar string
	for i := 0; i < size; i++ {
		str := string(input[i])
		if (str == " " && prevChar != " ") || !isDelim(str) {
			temp += str
			prevChar = str
		} else if prevChar != " " && isDelim(str) {
			temp += " "
		}
	}
	return strings.TrimSpace(strings.ToLower(temp))
}
