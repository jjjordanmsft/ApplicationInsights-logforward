package main

import (
	"bytes"
	"fmt"
	"regexp"
)

type regexpList []*regexp.Regexp

func (lst *regexpList) String() string {
	if len(*lst) == 0 {
		return ""
	}

	var buf bytes.Buffer
	for _, r := range *lst {
		if buf.Len() > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "\"%q\"", *r)
	}

	return buf.String()
}

func (lst *regexpList) Set(value string) error {
	r, err := regexp.Compile(value)
	if err != nil {
		return err
	}

	*lst = append(*lst, r)
	return nil
}

func (lst *regexpList) MatchAny(line string, dflt bool) bool {
	if len(*lst) == 0 {
		return dflt
	}

	for _, r := range *lst {
		if r.MatchString(line) {
			return true
		}
	}

	return false
}
