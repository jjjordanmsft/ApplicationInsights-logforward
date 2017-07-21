package common

import (
    "bytes"
    "fmt"
    "regexp"
    "testing"
)

type testParserCallbacks struct {}

var (
    //escPattern = `(\\[nftbrv"]|\\x[0-9]{2}|\\u[0-9]{2,4})`
    escPattern = `\\([nftbrv\"\\]|x[0-9a-fA-F]{2}|u[0-9a-fA-F]{4})`
    escRE = regexp.MustCompile(escPattern)
    escapes = map[byte]string{
        '"': "\"",
        '\\': "\\",
        'n': "\n",
        't': "\t",
        'r': "\r",
        'f': "\f",
        'b': "\b",
        'v': "\v",
    }
)

func (_ *testParserCallbacks) UnescapeValue(value string) string {
    var buf bytes.Buffer
    
    ptr := 0
    for _, run := range escRE.FindAllStringIndex(value, -1) {
        fmt.Println("HIT")
        buf.WriteString(value[ptr:run[0]])
        esc := value[run[0]:run[1]]
        if rep, ok := escapes[esc[1]]; ok {
            buf.WriteString(rep)
        } else if esc[1] == 'u' {
            buf.WriteString("?")
        } else if esc[1] == 'x' {
            buf.WriteString("?")
        }
        
        ptr = run[1]
    }
    
    buf.WriteString(value[ptr:])
    
    return buf.String()
}

func (_ *testParserCallbacks) UnwrapVariable(variable string) string {
    return variable
}

func makeOptions() *ParserOptions {
    return &ParserOptions{
        VariableRegex: `\$\{[^}]+\}`,
        EscapeRegex: escPattern,
        Callbacks: &testParserCallbacks{},
    }
}

func TestBoyerMoore(t *testing.T) {
    cb := &testParserCallbacks{}
    fmt.Printf("Unescape: %s\n", cb.UnescapeValue(`This\tis \"some\" text\nAnd another line!\u00ff\xff`))
    
    parser, _ := MakeParser("${front} - ${first} - ${second}", makeOptions())
    Show(parser.Parse("1 - 2 -3-  - 3"))
    
    parser, _ = MakeParser(`"${first}" "${second}" "${third}"`, makeOptions())
    Show(parser.Parse(`"this is" "some \" " "\"Text!\""`))
    Show(parser.Parse(`"this is" "some " "Text!"`))
}

func Show(val map[string]string, err error) {
    if err != nil {
        fmt.Printf("Error: %s\n", err.Error())
    } else {
        var buf bytes.Buffer
        for k, v := range(val) {
            fmt.Fprintf(&buf, "%s: %s\n", k, v)
        }
        fmt.Println(buf.String())
    }
}
