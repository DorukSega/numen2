package main

import (
	"fmt"
	"strconv"
	"sync"
	"unicode"
)

// Parser Token Type
type PType int // parser type
const (
	P_INT PType = iota
	P_FLOAT
	P_STRING
	P_BOOLEAN
	P_BLOCK // {}
	P_STACK // ()
	P_TYPE_LITERAL
	P_SYMBOL
)

var PTypeName = map[PType]string{
	P_INT:          "Integer",
	P_FLOAT:        "Float",
	P_STRING:       "String",
	P_BOOLEAN:      "Boolean",
	P_BLOCK:        "Block",
	P_STACK:        "Stack",
	P_TYPE_LITERAL: "Type Literal",
	P_SYMBOL:       "Symbol",
}

func (tp PType) String() string {
	return PTypeName[tp]
}

// TYPE LITERALS
type TypeLiterals int // type literals
const (
	TL_INT TypeLiterals = iota
	TL_FLOAT
	TL_STRING
	TL_BOOLEAN
	TL_BLOCK
	TL_STACK
)

var TypeLiteralStr = map[TypeLiterals]string{
	TL_INT:     "int",
	TL_FLOAT:   "float",
	TL_STRING:  "str",
	TL_BOOLEAN: "bool",
	TL_BLOCK:   "block",
	TL_STACK:   "stack",
}

func (tl TypeLiterals) String() string {
	return TypeLiteralStr[tl]
}

// Parser State
type PState int // parser state
const (
	PS_PARSING PState = iota
	PS_PROCEDURE
	PS_STRING
	PS_BLOCK
)

// Parser Token
type PToken struct {
	Type  PType
	Value any
}

func (token PToken) String() (result string) {
	if token.Type == P_STACK {
		result += fmt.Sprintf("<%v (", token.Type)
		for ix, tok := range (token.Value).([]PToken) {
			if ix > 0 {
				result += " "
			}
			result += fmt.Sprintf("%v", tok)
		}
		result += ")>"
	} else if token.Type == P_STRING {
		result = fmt.Sprintf("<%v \"%v\">", token.Type, token.Value)
	} else {
		result = fmt.Sprintf("<%v %v>", token.Type, token.Value)
	}
	return result
}

type IStack []PToken

type IScope struct {
	Scope map[string]any
	Mut   sync.RWMutex
}

type DeclaredFunction struct {
	Name           string
	CodeBlock      string
	ParameterStack IStack
	ReturnStack    IStack
}

type BuiltinFunction struct {
	Name           string
	ParameterStack IStack
	ReturnStack    IStack
	fn             func(*IStack)
}

var globalScope = IScope{}

func accessScopeObject(custom_scope *IScope, object_name string) (result any) {
	globalScope.Mut.RLock()
	result = globalScope.Scope[object_name]
	globalScope.Mut.RUnlock()
	if result != nil {
		return result
	}
	custom_scope.Mut.RLock()
	result = custom_scope.Scope[object_name]
	globalScope.Mut.RUnlock()
	if result != nil {
		fmt.Printf("[ACCS] No %v in scope!\n", object_name)
	}
	return result
	//panic("No value in scope")
}

func parser(code string, interp_chan chan PToken, wg *sync.WaitGroup) {
	defer wg.Done()
	var current_state = PS_PARSING
	var word []rune
	// how deep is the parser
	var block_deepness = 0
	var stack_deepness = 0
	// checks for fast parsing
	var has_digit = false
	var has_dot = false
	reset_all := func() {
		has_digit = false
		has_dot = false
		word = []rune{}
		current_state = PS_PARSING
		block_deepness = 0
		stack_deepness = 0
	}
	append_fast := func(char rune) {
		if unicode.IsSpace(char) {
			return
		}
		//if unicode.IsLetter(char) {
		//	has_letter = true
		//} else
		if unicode.IsDigit(char) {
			has_digit = true
		} else if char == '.' {
			has_dot = true
		}
		word = append(word, char)
	}
	for ix, char := range code {
		if current_state == PS_PARSING {
			var token_value any
			var token_type PType
			var parse_err error
			var end_of_code = false
			if ix == len(code)-1 {
				append_fast(char)
				end_of_code = true
			}
			if unicode.IsSpace(char) || end_of_code {
				if len(word) == 0 {
					continue
				}
				if has_digit {
					if has_dot {
						token_value, parse_err = strconv.ParseFloat(string(word), 64)
						if parse_err != nil {
							fmt.Println("[PRSR]: ", parse_err)
							return
						}
						token_type = P_FLOAT
					} else {
						token_value, parse_err = strconv.ParseInt(string(word), 0, 0)
						if parse_err != nil {
							fmt.Println("[PRSR]: ", parse_err)
							return
						}
						token_type = P_INT
					}
				} else {
					if token_value, parse_err = strconv.ParseBool(string(word)); parse_err == nil {
						token_type = P_BOOLEAN
					} else if string(word) == "int" || string(word) == "INT" {
						token_value = TL_INT
						token_type = P_TYPE_LITERAL
					} else if string(word) == "float" || string(word) == "FLOAT" {
						token_value = TL_FLOAT
						token_type = P_TYPE_LITERAL
					} else if string(word) == "str" || string(word) == "STR" {
						token_value = TL_STRING
						token_type = P_TYPE_LITERAL
					} else if string(word) == "bool" || string(word) == "BOOL" {
						token_value = TL_BOOLEAN
						token_type = P_TYPE_LITERAL
					} else if string(word) == "block" || string(word) == "BLOCK" {
						token_value = TL_BLOCK
						token_type = P_TYPE_LITERAL
					} else if string(word) == "stack" || string(word) == "STACK" {
						token_value = TL_STACK
						token_type = P_TYPE_LITERAL
						// todo: type literal could also be a type?
					} else { // todo: only accept a limited set
						token_value = string(word)
						token_type = P_SYMBOL
					}
				}
				if token_value == nil {
					fmt.Println("[PRSR]: Unable to parse ", word)
				}
				// build a token
				// send token
				interp_chan <- PToken{
					Value: token_value,
					Type:  token_type,
				}
				// clear word
				reset_all()
			} else if char == '{' {
				current_state = PS_BLOCK
				block_deepness = 1
			} else if char == '(' {
				current_state = PS_PROCEDURE
				stack_deepness = 1
			} else if char == '"' {
				current_state = PS_STRING
			} else {
				append_fast(char)
			}
		} else if current_state == PS_STRING {
			if char == '"' {
				if word[len(word)-1] == '\\' {
					word[len(word)-1] = char // char = '"'
				} else {
					// build a token
					// send token
					interp_chan <- PToken{
						Value: string(word),
						Type:  P_STRING,
					}
					// clear word
					reset_all()
				}
			} else {
				word = append(word, char)
			}
		} else if current_state == PS_PROCEDURE {
			if char == ')' {
				if stack_deepness > 1 {
					stack_deepness -= 1
					word = append(word, char)
				} else {
					// build a token
					// send token
					test := parser_collect(string(word))
					interp_chan <- PToken{
						Value: test,
						Type:  P_STACK,
					}
					// clear word
					reset_all()
				}
			} else {
				if char == '(' {
					stack_deepness += 1
				}
				word = append(word, char)
			}
		} else if current_state == PS_BLOCK {
			if char == '}' {
				if block_deepness > 1 {
					block_deepness -= 1
					word = append(word, char)
				} else {
					// build a token
					// send token
					interp_chan <- PToken{
						Value: string(word),
						Type:  P_BLOCK,
					}
					// clear word
					reset_all()
				}
			} else {
				if char == '{' {
					block_deepness += 1
				}
				word = append(word, char)
			}
		}
	}
	if current_state != PS_PARSING {
		if block_deepness > 0 {
			panic("[PRSR]: Block never closed, might be a missing '}'")
		} else if stack_deepness > 0 {
			panic("[PRSR]: Stack never closed, might be a missing ')'")
		} else if current_state == PS_STRING {
			panic("[PRSR]: String never closed, might be a missing '\"'")
		} else {
			panic("[PRSR]: TODO Parsing Error")
		}
	}
	close(interp_chan)
}

// returns all the parsed values instead
func parser_collect(code string) (parsed_tokens []PToken) {
	interp_chan := make(chan PToken)
	var wg sync.WaitGroup
	wg.Add(1)
	go parser(code, interp_chan, &wg)
	for token := range interp_chan {
		parsed_tokens = append(parsed_tokens, token)
	}
	wg.Wait()
	return parsed_tokens
}

func main() {
	code := `5 4 +`
	interp_chan := make(chan PToken)
	var wg sync.WaitGroup
	wg.Add(1)
	go parser(code, interp_chan, &wg)
	for token := range interp_chan {
		fmt.Printf("%v\n", token)
	}
	wg.Wait()
}
