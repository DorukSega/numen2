package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	P_BLOCK  // {}
	P_STACK  // ()
	P_SCHEMA // []
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
	P_SCHEMA:       "Schema",
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
	TL_SCHEMA
	TL_ANY
)

var TypeLiteralStr = map[TypeLiterals]string{
	TL_INT:     "int",
	TL_FLOAT:   "float",
	TL_STRING:  "str",
	TL_BOOLEAN: "bool",
	TL_BLOCK:   "block",
	TL_STACK:   "stack",
	TL_SCHEMA:  "schema",
	TL_ANY:     "any",
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
	PS_SCHEMA
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

// Scope Object Type
type SType int

const (
	S_Function_Builtin SType = iota
	S_Function_Declared
)

var STypeName = map[SType]string{
	S_Function_Builtin:  "Builtin Function",
	S_Function_Declared: "Declared Function",
}

func (sp SType) String() string {
	return STypeName[sp]
}

type ScopeObject struct {
	Object any
	Type   SType
	Mut    sync.RWMutex
}

type IScope map[string]*ScopeObject

type DeclaredFunction struct {
	Name           string
	ParameterStack IStack
	CodeBlock      string
}

type BuiltinFunction struct {
	Name           string
	ParameterStack IStack
	Fn             func(*IStack)
}

func fast_tlit(Value TypeLiterals) PToken {
	return PToken{Type: P_TYPE_LITERAL, Value: Value}
}

var globalScope = IScope{
	"print": { // todo: print should be part of std:io
		Type: S_Function_Builtin,
		Object: BuiltinFunction{
			Name:           "print",
			ParameterStack: IStack{fast_tlit(TL_ANY)},
			Fn: func(istack *IStack) {
				// print
				tok := istack.PopAny()
				if !Contains(tok.Type, []PType{P_INT, P_FLOAT, P_BOOLEAN, P_STRING}) {
					panic(fmt.Sprintf("[PRT] can't print %v type", tok.Type))
				}
				fmt.Println(tok.Value)
			},
		},
		Mut: sync.RWMutex{},
	},
	"+": {
		Type: S_Function_Builtin,
		Object: BuiltinFunction{
			Name:           "add",
			ParameterStack: IStack{fast_tlit(TL_ANY), fast_tlit(TL_ANY)},
			Fn: func(istack *IStack) {
				first := istack.PopAny()
				second := istack.PopAny()
				var result any
				var result_type PType
				// add panics for failed casting
				if first.Type == P_INT {
					fvalue := first.Value.(int64)
					if second.Type == P_INT {
						svalue := second.Value.(int64)
						result = svalue + fvalue
						result_type = P_INT
					} else if second.Type == P_FLOAT {
						svalue := second.Value.(float64)
						result = svalue + float64(fvalue)
						result_type = P_FLOAT
					}
				} else if first.Type == P_FLOAT {
					fvalue := first.Value.(float64)
					if second.Type == P_INT {
						svalue := second.Value.(int64)
						result = float64(svalue) + fvalue
						result_type = P_FLOAT
					} else if second.Type == P_FLOAT {
						svalue := second.Value.(float64)
						result = svalue + fvalue
						result_type = P_FLOAT
					}
				} else if first.Type == P_STRING && second.Type == P_STRING {
					fvalue := first.Value.(string)
					svalue := second.Value.(string)
					result = svalue + fvalue
					result_type = P_STRING
				}
				if result == nil {
					panic(fmt.Sprintf("[ADD] unexpected types %v - %v", first.Type, second.Type))
				}
				*istack = append(*istack, PToken{
					Value: result,
					Type:  result_type,
				})
			},
		},
		Mut: sync.RWMutex{},
	},
}

func readScopeObject(object_name string, local_scope *IScope) (object *ScopeObject, ok bool) {
	if gobject := globalScope[object_name]; gobject != nil {
		return gobject, true
	}
	if lobject := (*local_scope)[object_name]; lobject != nil {
		return lobject, true
	}
	return nil, false
}

// func readScopeObject(custom_scope *IScope, object_name string) (result any) {
// 	globalScope.Mut.RLock()
// 	result = globalScope.Scope[object_name]
// 	globalScope.Mut.RUnlock()
// 	if result != nil {
// 		return result
// 	}
// 	custom_scope.Mut.RLock()
// 	result = custom_scope.Scope[object_name]
// 	globalScope.Mut.RUnlock()
// 	if result == nil {
// 		fmt.Printf("[ACCS] No %v in scope!\n", object_name)
// 	}
// 	return result
// }

//func writeScopeObject(custom_Scope *IScope, object_name string, value any) {
//}

func parser(code string, interp_chan chan PToken, wg *sync.WaitGroup) {
	defer wg.Done()
	var current_state = PS_PARSING
	var word []rune
	// how deep is the parser
	var block_deepness = 0
	var stack_deepness = 0
	var schema_deepness = 0
	// checks for fast parsing
	var has_digit = false
	var has_letter = false
	var has_dot = false
	reset_all := func() {
		has_digit = false
		has_dot = false
		has_letter = false
		word = []rune{}
		current_state = PS_PARSING
		block_deepness = 0
		stack_deepness = 0
		schema_deepness = 0
	}
	append_fast := func(char rune) {
		if unicode.IsSpace(char) {
			return
		}
		if unicode.IsLetter(char) {
			has_letter = true
		} else if unicode.IsDigit(char) {
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
				if has_digit && !has_letter {
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
					} else if string(word) == "any" || string(word) == "ANY" {
						token_value = TL_ANY
						token_type = P_TYPE_LITERAL
					} else if string(word) == "block" || string(word) == "BLOCK" {
						token_value = TL_BLOCK
						token_type = P_TYPE_LITERAL
					} else if string(word) == "stack" || string(word) == "STACK" {
						token_value = TL_STACK
						token_type = P_TYPE_LITERAL
					} else if string(word) == "schema" || string(word) == "SCHEMA" {
						token_value = TL_SCHEMA
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
			} else if char == '[' {
				current_state = PS_SCHEMA
				schema_deepness = 1
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
					parsed := parser_collect(string(word))
					interp_chan <- PToken{
						Value: parsed,
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
					cblock := strings.TrimSpace(string(word))
					interp_chan <- PToken{
						Value: string(cblock),
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
		} else if current_state == PS_SCHEMA {
			// todo write this
			schema_deepness += 1
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

func interpret(interp_chan chan PToken, wg *sync.WaitGroup, local_scope *IScope, local_stack *IStack) {
	defer wg.Done()
	if local_scope == nil {
		local_scope = &IScope{}
	}
	if local_stack == nil {
		local_stack = &IStack{}
	}
	for token := range interp_chan {
		t_type := token.Type
		t_value := token.Value
		if t_type == P_SYMBOL {
			t_parsed := t_value.(string) //todo: add custom error handling
			if scope_object, ok := readScopeObject(t_parsed, local_scope); ok {
				scope_object.Mut.RLock()
				if scope_object.Type == S_Function_Builtin {
					t_builtin := scope_object.Object.(BuiltinFunction)
					p_stack := t_builtin.ParameterStack
					if len(*local_stack) < len(p_stack) {
						panic(fmt.Sprintf("[INTR] not enough stack items to call %v function.\nNeeded %v, got %v.",
							t_builtin.Name, len(p_stack), len(*local_stack)))
					}
					// check parameters:
					// - they should all be a type literals
					// call function
					t_builtin.Fn(local_stack)
					// create a function that safe gathers them?
				}
				scope_object.Mut.RUnlock()
			}
		} else {
			*local_stack = append(*local_stack, token)
		}

	}
}

func main() {
	code_raw, _ := os.ReadFile("./test.nm")
	code := string(code_raw)
	interp_chan := make(chan PToken)
	var wg sync.WaitGroup
	wg.Add(2)
	go parser(code, interp_chan, &wg)
	go interpret(interp_chan, &wg, &globalScope, nil)
	wg.Wait()
}
