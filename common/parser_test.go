package common

import (
    "strconv"
    "testing"
)

func NewTestParser(t *testing.T, format string) *Parser {
    p, err := NewParser(format, &ParserOptions{
        VariableRegex: `\$[0-9]`,
        EscapeRegex: UnescapeCommonPattern,
        Unescape: UnescapeCommon,
        UnwrapVariable: func (v string) string { return v[1:] },
    })
    
    if err != nil {
        t.Fatal("Parser constructor failed: %s", err.Error())
    }
    
    return p
}

type testParseResult []string

func (result *testParseResult) Store(key, value string) {
    if n, err := strconv.Atoi(key); err == nil {
        for len(*result) <= n {
            *result = append(*result, "")
        }
    
        (*result)[n] = value
    }
}

func parseTest(t *testing.T, parser *Parser, line string, expected ...string) {
    r := make(testParseResult, 0)
    err := parser.Parse(line, &r)
    if err != nil {
        t.Errorf("Parser.Parse failed: %s", err.Error())
    } else {
        if len(expected) != len(r) {
            t.Error("Output does not match expected length")
        } else {
            for i := range expected {
                if expected[i] != r[i] {
                    t.Errorf("Mismatch at %d. Actual: \"%s\" Expected: \"%s\"", i, r[i], expected[i])
                }
            }
        }
    }
}

func parseTestError(t *testing.T, parser *Parser, line string) {
    r := make(testParseResult, 0)
    err := parser.Parse(line, &r)
    if err == nil {
        t.Errorf("Parser.Parse should not have succeeded: %s", line)
    }
}

func TestBoyerMoore(t *testing.T) {
    parser := NewTestParser(t, "$0 $1 $2")
    parseTest(t, parser, "a b ", "a", "b", "")
    parseTest(t, parser, "a b c", "a", "b", "c")
    parseTest(t, parser, "a b c d e", "a", "b", "c d e")

    parser = NewTestParser(t, "$0 - $1 - $2")
    parseTest(t, parser, "1 - 2 -3-  - 3", "1", "2 -3- ", "3")
    parseTest(t, parser, `\n - \t - \r`, "\n", "\t", "\r")
    
    parser = NewTestParser(t, `"$0" "$1" "$2"`)
    parseTest(t, parser, `"" "" ""`, "", "", "")
    parseTest(t, parser, `"first" "second" "third"`, "first", "second", "third")
    parseTest(t, parser, `"this is" "some \" " "\"Text!\""`, `this is`, `some " `, `"Text!"`)
    parseTest(t, parser, `junk "first" "second" "third" junk`, "first", "second", "third")
    parseTest(t, parser, `"How about" "\"\"\"\" \" \" \" \"  \"  \"  \"" "Some quotes? (\")"`, "How about", `"""" " " " "  "  "  "`, `Some quotes? (")`)
}

func TestParseErrors(t *testing.T) {
    parser := NewTestParser(t, `$0 $1 $2`)
    parseTestError(t, parser, "a")
    parseTestError(t, parser, "a b")
    parseTestError(t, parser, "a\t")
    
    parser = NewTestParser(t, `"$0" "$1" "$2"`)
    parseTestError(t, parser, `"1", "2", "3"`)
    parseTestError(t, parser, `"1" "2" "3`)
    parseTestError(t, parser, `1" "2" "3"`)
}
