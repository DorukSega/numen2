package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
	TL_SYMBOL
	TL_ANY
)

var TypeLiteralStr = map[TypeLiterals]string{
	TL_INT:     "int",
	TL_FLOAT:   "float",
	TL_STRING:  "str",
	TL_BOOLEAN: "bool",
	TL_BLOCK:   "block",
	TL_STACK:   "stack",
	TL_SYMBOL:  "symbol",
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
	PS_LINE_COMMENT
	PS_BLOCK_COMMENT
)

// Parser Token
type PToken struct {
	Type  PType
	Value any
}

func (token PToken) String() (result string) {
	if token.Type == P_STACK {
		result += fmt.Sprintf("<%v (", token.Type)
		for ix, tok := range (token.Value).(IStack) {
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
	Name            string
	ParameterStack  IStack
	CodeBlock       string
	ParsedCodeBlock *IStack // only parse once for performance
}

type BuiltinFunction struct {
	Name           string
	ParameterStack IStack
	Fn             func(*IStack, *IScope)
}

// turn given type literals to ParameterStack
func fast_pstack(values ...TypeLiterals) (stack IStack) {
	for _, value := range values {
		stack = append(stack, PToken{P_TYPE_LITERAL, value})
	}
	return stack
}

type ModuleCompositeKey struct {
	Parent  string // folder1/folder2 of folder1/folder2/file or std of std:math
	RefName string // reference name defined by user or file
}

var modules = map[string]*IScope{
	"sys": {
		"write": {
			Type: S_Function_Builtin,
			Object: BuiltinFunction{
				Name:           "write",
				ParameterStack: fast_pstack(TL_STRING, TL_INT),
				Fn: func(istack *IStack, customScope *IScope) {
					fd := istack.PopInt()
					text := istack.PopString()
					syscall.Write(int(fd), []byte(text))
				},
			},
			Mut: sync.RWMutex{},
		},
	},
}

// todo: Pop functions should propogate errors and handled here
var globalScope = IScope{
	"fun": {
		Type: S_Function_Builtin,
		Object: BuiltinFunction{
			Name:           "fun",
			ParameterStack: fast_pstack(TL_SYMBOL, TL_STACK, TL_BLOCK),
			Fn: func(istack *IStack, customScope *IScope) {
				code_block := istack.PopBlock()
				// todo: validate parameter stack? maybe do this only at first call
				parameter_stack := istack.PopStack()
				// todo: don't accept all function names
				fname := istack.PopString()

				writeScopeObject(customScope, fname, &ScopeObject{
					Type: S_Function_Declared,
					Object: DeclaredFunction{
						Name:            fname,
						ParameterStack:  parameter_stack,
						CodeBlock:       code_block,
						ParsedCodeBlock: nil,
					},
					Mut: sync.RWMutex{},
				})

			},
		},
		Mut: sync.RWMutex{},
	},
	"+": {
		Type: S_Function_Builtin,
		Object: BuiltinFunction{
			Name:           "add",
			ParameterStack: fast_pstack(TL_ANY, TL_ANY),
			Fn: func(istack *IStack, customScope *IScope) {
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

func readScopeObject(object_name string, local_scope *IScope, skip_global bool) (object *ScopeObject, ok bool) {
	if !skip_global {
		if gobject := globalScope[object_name]; gobject != nil {
			return gobject, true
		}
	}
	if lobject := (*local_scope)[object_name]; lobject != nil {
		return lobject, true
	}
	return nil, false
}

func writeScopeObject(custom_Scope *IScope, object_name string, value *ScopeObject) {
	(*custom_Scope)[object_name] = value
}

func parser(code string, interp_chan chan PToken, wg *sync.WaitGroup, parsed_code_collect *IStack) {
	defer wg.Done()
	var current_state = PS_PARSING
	var word []rune
	// how deep is the parser
	var block_deepness = 0
	var stack_deepness = 0
	// checks for fast parsing
	var has_digit = false
	var has_letter = false
	var has_dot = false
	// for comments
	var first_comment_slash = false // first /
	var closing_star = false        // closing * of */
	// string escapes
	var first_string_escape = false // first \
	reset_all := func() {
		has_digit = false
		has_dot = false
		has_letter = false
		first_comment_slash = false
		first_string_escape = false
		closing_star = false
		word = []rune{}
		current_state = PS_PARSING
		block_deepness = 0
		stack_deepness = 0
	}
	push_value := func(token_value any, token_type PType) {
		ptoken := PToken{
			Value: token_value,
			Type:  token_type,
		}
		interp_chan <- ptoken
		if parsed_code_collect != nil {
			*parsed_code_collect = append(*parsed_code_collect, ptoken)
		}
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

	parse_word := func() {
		var token_value any
		var token_type PType
		var parse_err error
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
		push_value(token_value, token_type)
		reset_all()
	}

	for ix, char := range code {
		if current_state == PS_PARSING {
			var end_of_code = false
			if ix == len(code)-1 {
				append_fast(char)
				end_of_code = true
			}
			if first_comment_slash {
				if char == '/' {
					first_comment_slash = false
					current_state = PS_LINE_COMMENT
				} else if char == '*' {
					first_comment_slash = false
					current_state = PS_BLOCK_COMMENT
				}
			}
			if unicode.IsSpace(char) || end_of_code {
				if len(word) == 0 {
					continue
				}
				parse_word()
			} else if char == '{' {
				if len(word) > 0 {
					parse_word()
				}
				current_state = PS_BLOCK
				block_deepness = 1
			} else if char == '(' {
				if len(word) > 0 {
					parse_word()
				}
				current_state = PS_PROCEDURE
				stack_deepness = 1
			} else if char == '"' {
				if len(word) > 0 {
					parse_word()
				}
				current_state = PS_STRING
			} else if char == '/' {
				first_comment_slash = true
			} else {
				append_fast(char)
			}
		} else if current_state == PS_LINE_COMMENT {
			// do nothing until new line
			if char == '\n' {
				reset_all()
			}
		} else if current_state == PS_BLOCK_COMMENT {
			// do nothing until */
			if char == '*' {
				closing_star = true
			} else if closing_star && char == '/' {
				reset_all()
			}
		} else if current_state == PS_STRING {
			if first_string_escape {
				last_index := len(word) - 1
				if char == '"' {
					first_string_escape = false
					word[last_index] = char
					continue
				} else if char == 'n' {
					first_string_escape = false
					word[last_index] = '\n'
					continue
				} else if char == 't' {
					first_string_escape = false
					word[last_index] = '\t'
					continue
				} else if char == '\\' {
					first_string_escape = false
					continue
				}
				// probably invalid do nothing
			}

			if char == '"' {
				if word[len(word)-1] == '\\' {
					word[len(word)-1] = char // char = '"'
				} else {
					// build a token
					// send token
					push_value(string(word), P_STRING)
					// clear word
					reset_all()
				}
			} else {
				if char == '\\' {
					first_string_escape = true
				}
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
					push_value(parsed, P_STACK)
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
					push_value(string(cblock), P_BLOCK)
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
	if !Contains(current_state, PS_PARSING, PS_LINE_COMMENT) {
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
func parser_collect(code string) (parsed_tokens IStack) {
	interp_chan := make(chan PToken)
	var wg sync.WaitGroup
	wg.Add(1)
	go parser(code, interp_chan, &wg, nil)
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
			if strings.ContainsRune(t_parsed, ':') {
				module_group := strings.Split(t_parsed, ":")
				if len(module_group) != 2 {
					panicf("Faulty module access %v", t_parsed)
				}
				module_name := module_group[0]
				scope_obj_name := module_group[1]
				mod_scope := modules[module_name]
				if mod_scope == nil {
					panicf("Module %v loaded, please load as such:\n`std:%v import`\n`parent_folder/%v import`",
						module_name, module_name, module_name)
				}
				scope_object, ok := readScopeObject(scope_obj_name, mod_scope, true)
				if !ok {
					panicf("Module has no scope object named %v", scope_obj_name)
				}
				// todo: turn this to a function since it is same both places
				if Contains(scope_object.Type, S_Function_Builtin, S_Function_Declared) {
					run_function(scope_object, local_scope, local_stack)
				}
				// todo: variables
			} else if scope_object, ok := readScopeObject(t_parsed, local_scope, false); ok {
				if Contains(scope_object.Type, S_Function_Builtin, S_Function_Declared) {
					run_function(scope_object, local_scope, local_stack)
				}
				// todo: variables
			} else {
				*local_stack = append(*local_stack, token)
			}
		} else {
			*local_stack = append(*local_stack, token)
		}
		//fmt.Println(token)
	}
	//fmt.Printf("%+v\n", (*local_scope)["main"].Object)
}

func run_function(function *ScopeObject, local_scope *IScope, local_stack *IStack) {
	var local_stack_size int
	if local_stack != nil {
		local_stack_size = len(*local_stack)
	}

	if function.Type == S_Function_Declared {
		function.Mut.Lock()
		declared := function.Object.(DeclaredFunction)
		p_stack := declared.ParameterStack
		var function_stack IStack

		if len(p_stack) > 0 {
			if local_stack_size < len(p_stack) {
				panic(fmt.Sprintf("[INTR] not enough stack items to call %v function.\nNeeded %v, got %v.",
					declared.Name, len(p_stack), len(*local_stack)))
			}

			for i := len(p_stack) - 1; i >= 0; i-- {
				parameter := p_stack[i]
				// todo: implement symbol handling, Contains(parameter.Type, P_TYPE_LITERAL, P_SYMBOL)
				assert(parameter.Type == P_TYPE_LITERAL, "[INTR] parameter can't be of type %v", parameter.Type)
				if parameter.Type == P_TYPE_LITERAL {
					tlit := parameter.Value.(TypeLiterals)
					val := local_stack.PopAny()
					switch tlit {
					case TL_INT:
						assert(val.Type == P_INT, "[INTR] expected type %v got %v", tlit, val.Type)
					case TL_FLOAT:
						assert(val.Type == P_FLOAT, "[INTR] expected type %v got %v", tlit, val.Type)
					case TL_STRING:
						assert(val.Type == P_STRING, "[INTR] expected type %v got %v", tlit, val.Type)
					case TL_BOOLEAN:
						assert(val.Type == P_BOOLEAN, "[INTR] expected type %v got %v", tlit, val.Type)
					case TL_BLOCK:
						assert(val.Type == P_BLOCK, "[INTR] expected type %v got %v", tlit, val.Type)
					case TL_STACK:
						assert(val.Type == P_STACK, "[INTR] expected type %v got %v", tlit, val.Type)
					case TL_SYMBOL:
						assert(val.Type == P_SYMBOL, "[INTR] expected type %v got %v", tlit, val.Type)
					case TL_ANY: // do no checks
					default:
						panicf("[INTR] unimplemented type %v", tlit)
					}
					function_stack.PushFront(val)
				}
			}
		}

		interp_chan := make(chan PToken)
		var wg sync.WaitGroup
		go interpret(interp_chan, &wg, local_scope, &function_stack)

		if parsed_code := declared.ParsedCodeBlock; parsed_code != nil {
			// if code is parsed already, don't parse
			wg.Add(1)
			go func() {
				for _, token := range *parsed_code {
					interp_chan <- token
				}
				close(interp_chan)
			}()
		} else { // parse first
			parsed_code = &IStack{}
			go parser(declared.CodeBlock, interp_chan, &wg, parsed_code)
			wg.Add(2)
		}
		wg.Wait()

		function.Mut.Unlock()
	} else if function.Type == S_Function_Builtin {
		function.Mut.RLock()
		t_builtin := function.Object.(BuiltinFunction)
		p_stack := t_builtin.ParameterStack
		if len(p_stack) > 0 {
			if local_stack_size < len(p_stack) {
				panic(fmt.Sprintf("[INTR] not enough stack items to call %v function.\nNeeded %v, got %v.",
					t_builtin.Name, len(p_stack), len(*local_stack)))
			}
		}
		// call function
		t_builtin.Fn(local_stack, local_scope)
		// create a function that safe gathers them?
		function.Mut.RUnlock()
	}
}

func load_module(rawPathToken PToken, refName string) {

	code_raw, _ := os.ReadFile("./test.nm")
	code := string(code_raw)

	interp_chan := make(chan PToken)
	var wg sync.WaitGroup
	wg.Add(2)
	go parser(code, interp_chan, &wg, nil)
	go interpret(interp_chan, &wg, &globalScope, nil)
	wg.Wait()
}

func main() {
	code_raw, _ := os.ReadFile("./test.nm")
	code := string(code_raw)

	interp_chan := make(chan PToken)
	var wg sync.WaitGroup
	wg.Add(2)
	go parser(code, interp_chan, &wg, nil)
	go interpret(interp_chan, &wg, &globalScope, nil)
	wg.Wait()

	if main_function := globalScope["main"]; main_function != nil {
		if main_function.Type == S_Function_Declared {
			run_function(main_function, nil, nil)
		}
	}
}
