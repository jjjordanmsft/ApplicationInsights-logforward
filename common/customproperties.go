package common

import (
    "fmt"
    "strings"
)

type customProperties map[string]string

func (props *customProperties) String() string {
    return fmt.Sprintf("%q", map[string]string(*props))
}

func (props *customProperties) Set(value string) error {
    eq := strings.IndexByte(value, '=')
    if eq < 0 {
        return fmt.Errorf("Invalid custom property (should be 'key=value')")
    }
    
    if *props == nil {
        *props = make(map[string]string)
    }
    
    (*props)[value[0:eq]] = value[eq + 1:len(value)]
    return nil
}
