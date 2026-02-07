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
	P_MEMORY // []
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
	P_MEMORY:       "Memory",
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
	TL_MEMORY
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
	TL_MEMORY:  "memory",
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
	PS_MEMORY
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

type IScope map[string]PToken

type loopContext struct {
	shouldBreak bool
}

var globalStack IStack
var globalScope = IScope{}
var loopStack []*loopContext
var builtins map[string]func()

func init() {
	builtins = map[string]func(){
		"dbgprint": func() {
			value := globalStack.PopAny()
			fmt.Printf("<%v, %v>\n", value.Type, value.Value)
			// Push it back so it doesn't consume the value
			globalStack = append(globalStack, value)
		},
		"+": func() {
			first := globalStack.PopAny()
			second := globalStack.PopAny()
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
			globalStack = append(globalStack, PToken{
				Value: result,
				Type:  result_type,
			})
		},
		"-": func() {
			first := globalStack.PopAny()
			second := globalStack.PopAny()
			var result any
			var result_type PType
			if first.Type == P_INT && second.Type == P_INT {
				result = second.Value.(int64) - first.Value.(int64)
				result_type = P_INT
			} else if first.Type == P_FLOAT && second.Type == P_FLOAT {
				result = second.Value.(float64) - first.Value.(float64)
				result_type = P_FLOAT
			} else if first.Type == P_INT && second.Type == P_FLOAT {
				result = second.Value.(float64) - float64(first.Value.(int64))
				result_type = P_FLOAT
			} else if first.Type == P_FLOAT && second.Type == P_INT {
				result = float64(second.Value.(int64)) - first.Value.(float64)
				result_type = P_FLOAT
			} else {
				panic(fmt.Sprintf("[SUB] unexpected types %v - %v", first.Type, second.Type))
			}
			globalStack = append(globalStack, PToken{Value: result, Type: result_type})
		},
		"*": func() {
			first := globalStack.PopAny()
			second := globalStack.PopAny()
			var result any
			var result_type PType
			if first.Type == P_INT && second.Type == P_INT {
				result = second.Value.(int64) * first.Value.(int64)
				result_type = P_INT
			} else if first.Type == P_FLOAT && second.Type == P_FLOAT {
				result = second.Value.(float64) * first.Value.(float64)
				result_type = P_FLOAT
			} else if first.Type == P_INT && second.Type == P_FLOAT {
				result = second.Value.(float64) * float64(first.Value.(int64))
				result_type = P_FLOAT
			} else if first.Type == P_FLOAT && second.Type == P_INT {
				result = float64(second.Value.(int64)) * first.Value.(float64)
				result_type = P_FLOAT
			} else {
				panic(fmt.Sprintf("[MUL] unexpected types %v - %v", first.Type, second.Type))
			}
			globalStack = append(globalStack, PToken{Value: result, Type: result_type})
		},
		"/": func() {
			first := globalStack.PopAny()
			second := globalStack.PopAny()
			var result any
			var result_type PType
			if first.Type == P_INT && second.Type == P_INT {
				if first.Value.(int64) == 0 {
					panic("[DIV] division by zero")
				}
				result = second.Value.(int64) / first.Value.(int64)
				result_type = P_INT
			} else if first.Type == P_FLOAT && second.Type == P_FLOAT {
				if first.Value.(float64) == 0 {
					panic("[DIV] division by zero")
				}
				result = second.Value.(float64) / first.Value.(float64)
				result_type = P_FLOAT
			} else if first.Type == P_INT && second.Type == P_FLOAT {
				if first.Value.(int64) == 0 {
					panic("[DIV] division by zero")
				}
				result = second.Value.(float64) / float64(first.Value.(int64))
				result_type = P_FLOAT
			} else if first.Type == P_FLOAT && second.Type == P_INT {
				if first.Value.(float64) == 0 {
					panic("[DIV] division by zero")
				}
				result = float64(second.Value.(int64)) / first.Value.(float64)
				result_type = P_FLOAT
			} else {
				panic(fmt.Sprintf("[DIV] unexpected types %v - %v", first.Type, second.Type))
			}
			globalStack = append(globalStack, PToken{Value: result, Type: result_type})
		},
		"store": func() {
			varname_token := globalStack.PopAny()
			assert(varname_token.Type == P_SYMBOL, "[STORE] variable name must be a symbol, got %v", varname_token.Type)
			varname := varname_token.Value.(string)
			value := globalStack.PopAny()
			globalScope[varname] = value
		},
		"load": func() {
			varname_token := globalStack.PopAny()
			assert(varname_token.Type == P_SYMBOL, "[LOAD] variable name must be a symbol, got %v", varname_token.Type)
			varname := varname_token.Value.(string)
			value, ok := globalScope[varname]
			if !ok {
				panicf("[LOAD] variable %v not found", varname)
			}
			globalStack = append(globalStack, value)
		},
		"run": func() {
			code_block := globalStack.PopBlock()
			run_function(code_block, nil)
		},
		"push": func() {
			// Syntax: value stack push
			stack := globalStack.PopStack()
			value := globalStack.PopAny()
			stack = append(stack, value)
			globalStack = append(globalStack, PToken{P_STACK, stack})
		},
		"pop": func() {
			stack := globalStack.PopStack()
			if len(stack) == 0 {
				panicf("[POP] cannot pop from empty stack")
			}
			value := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			globalStack = append(globalStack, PToken{P_STACK, stack})
			globalStack = append(globalStack, value)
		},
		"swap": func() {
			// ( a b -- b a )
			b := globalStack.PopAny()
			a := globalStack.PopAny()
			globalStack = append(globalStack, b)
			globalStack = append(globalStack, a)
		},
		"rot": func() {
			// ( a b c -- b c a )
			c := globalStack.PopAny()
			b := globalStack.PopAny()
			a := globalStack.PopAny()
			globalStack = append(globalStack, b)
			globalStack = append(globalStack, c)
			globalStack = append(globalStack, a)
		},
		"dup": func() {
			// ( a -- a a )
			a := globalStack.PopAny()
			globalStack = append(globalStack, a)
			globalStack = append(globalStack, a)
		},
		"drop": func() {
			// ( a -- )
			globalStack.PopAny()
		},
		"over": func() {
			// ( a b -- a b a )
			if len(globalStack) < 2 {
				panicf("[OVER] need at least 2 items on stack")
			}
			a := globalStack[len(globalStack)-2]
			globalStack = append(globalStack, a)
		},
		"storeto": func() {
			// Syntax: value key mem storeto
			// Pop memory first (top), then key, then value (bottom)
			mem_or_sym := globalStack.PopAny()
			key_token := globalStack.PopAny()
			assert(key_token.Type == P_SYMBOL, "[STORETO] key must be a symbol, got %v", key_token.Type)
			key := key_token.Value.(string)
			value := globalStack.PopAny()

			var memory IMemory
			if mem_or_sym.Type == P_SYMBOL {
				// Load from scope: value key memsym storeto
				varname := mem_or_sym.Value.(string)
				mem_token, ok := globalScope[varname]
				if !ok {
					panicf("[STORETO] variable %v not found", varname)
				}
				assert(mem_token.Type == P_MEMORY, "[STORETO] %v is not a memory, got %v", varname, mem_token.Type)
				memory = mem_token.Value.(IMemory)
			} else if mem_or_sym.Type == P_MEMORY {
				// Direct memory: value key [] storeto
				memory = mem_or_sym.Value.(IMemory)
			} else {
				panicf("[STORETO] expected symbol or memory, got %v", mem_or_sym.Type)
			}

			// Create new memory (immutable)
			new_memory := make(IMemory)
			for k, v := range memory {
				new_memory[k] = v
			}
			new_memory[key] = value

			globalStack = append(globalStack, PToken{P_MEMORY, new_memory})
		},
		"loadfrom": func() {
			// Pop memory/symbol first (top of stack), then key
			mem_or_sym := globalStack.PopAny()
			key_token := globalStack.PopAny()
			assert(key_token.Type == P_SYMBOL, "[LOADFROM] key must be a symbol, got %v", key_token.Type)
			key := key_token.Value.(string)

			var memory IMemory

			if mem_or_sym.Type == P_SYMBOL {
				// Load from scope first: a mymem loadfrom
				varname := mem_or_sym.Value.(string)
				mem_token, ok := globalScope[varname]
				if !ok {
					panicf("[LOADFROM] variable %v not found", varname)
				}
				assert(mem_token.Type == P_MEMORY, "[LOADFROM] %v is not a memory, got %v", varname, mem_token.Type)
				memory = mem_token.Value.(IMemory)
			} else if mem_or_sym.Type == P_MEMORY {
				// Direct memory: a [] loadfrom
				memory = mem_or_sym.Value.(IMemory)
			} else {
				panicf("[LOADFROM] expected symbol or memory, got %v", mem_or_sym.Type)
			}

			value, ok := memory[key]
			if !ok {
				panicf("[LOADFROM] key %v not found in memory", key)
			}

			globalStack = append(globalStack, value)
		},
		"call": func() {
			// Syntax: funcmem call  or  funcsym call
			func_or_sym := globalStack.PopAny()

			var function IMemory
			if func_or_sym.Type == P_SYMBOL {
				// Load from scope: sumfunc call
				varname := func_or_sym.Value.(string)
				func_token, ok := globalScope[varname]
				if !ok {
					panicf("[CALL] variable %v not found", varname)
				}
				assert(func_token.Type == P_MEMORY, "[CALL] %v is not a memory, got %v", varname, func_token.Type)
				function = func_token.Value.(IMemory)
			} else if func_or_sym.Type == P_MEMORY {
				// Direct memory
				function = func_or_sym.Value.(IMemory)
			} else {
				panicf("[CALL] expected symbol or memory, got %v", func_or_sym.Type)
			}

			// Check for params (ignore for now)
			if _, has_params := function["params"]; has_params {
				// TODO: handle params later
			}

			// Get code block
			code_token, ok := function["code"]
			if !ok {
				panicf("[CALL] function has no 'code' key")
			}
			assert(code_token.Type == P_BLOCK, "[CALL] 'code' must be a block, got %v", code_token.Type)
			code_block := code_token.Value.(string)

			// Run the code
			run_function(code_block, nil)
		},
		"<": func() {
			second := globalStack.PopAny()
			first := globalStack.PopAny()
			var result bool
			if first.Type == P_INT && second.Type == P_INT {
				result = first.Value.(int64) < second.Value.(int64)
			} else if first.Type == P_FLOAT && second.Type == P_FLOAT {
				result = first.Value.(float64) < second.Value.(float64)
			} else if first.Type == P_INT && second.Type == P_FLOAT {
				result = float64(first.Value.(int64)) < second.Value.(float64)
			} else if first.Type == P_FLOAT && second.Type == P_INT {
				result = first.Value.(float64) < float64(second.Value.(int64))
			} else {
				panicf("[<] unexpected types %v - %v", first.Type, second.Type)
			}
			globalStack = append(globalStack, PToken{P_BOOLEAN, result})
		},
		">": func() {
			second := globalStack.PopAny()
			first := globalStack.PopAny()
			var result bool
			if first.Type == P_INT && second.Type == P_INT {
				result = first.Value.(int64) > second.Value.(int64)
			} else if first.Type == P_FLOAT && second.Type == P_FLOAT {
				result = first.Value.(float64) > second.Value.(float64)
			} else if first.Type == P_INT && second.Type == P_FLOAT {
				result = float64(first.Value.(int64)) > second.Value.(float64)
			} else if first.Type == P_FLOAT && second.Type == P_INT {
				result = first.Value.(float64) > float64(second.Value.(int64))
			} else {
				panicf("[>] unexpected types %v - %v", first.Type, second.Type)
			}
			globalStack = append(globalStack, PToken{P_BOOLEAN, result})
		},
		"<=": func() {
			second := globalStack.PopAny()
			first := globalStack.PopAny()
			var result bool
			if first.Type == P_INT && second.Type == P_INT {
				result = first.Value.(int64) <= second.Value.(int64)
			} else if first.Type == P_FLOAT && second.Type == P_FLOAT {
				result = first.Value.(float64) <= second.Value.(float64)
			} else if first.Type == P_INT && second.Type == P_FLOAT {
				result = float64(first.Value.(int64)) <= second.Value.(float64)
			} else if first.Type == P_FLOAT && second.Type == P_INT {
				result = first.Value.(float64) <= float64(second.Value.(int64))
			} else {
				panicf("[<=] unexpected types %v - %v", first.Type, second.Type)
			}
			globalStack = append(globalStack, PToken{P_BOOLEAN, result})
		},
		">=": func() {
			second := globalStack.PopAny()
			first := globalStack.PopAny()
			var result bool
			if first.Type == P_INT && second.Type == P_INT {
				result = first.Value.(int64) >= second.Value.(int64)
			} else if first.Type == P_FLOAT && second.Type == P_FLOAT {
				result = first.Value.(float64) >= second.Value.(float64)
			} else if first.Type == P_INT && second.Type == P_FLOAT {
				result = float64(first.Value.(int64)) >= second.Value.(float64)
			} else if first.Type == P_FLOAT && second.Type == P_INT {
				result = first.Value.(float64) >= float64(second.Value.(int64))
			} else {
				panicf("[>=] unexpected types %v - %v", first.Type, second.Type)
			}
			globalStack = append(globalStack, PToken{P_BOOLEAN, result})
		},
		"==": func() {
			second := globalStack.PopAny()
			first := globalStack.PopAny()
			var result bool
			if first.Type != second.Type {
				result = false
			} else if first.Type == P_INT {
				result = first.Value.(int64) == second.Value.(int64)
			} else if first.Type == P_FLOAT {
				result = first.Value.(float64) == second.Value.(float64)
			} else if first.Type == P_STRING {
				result = first.Value.(string) == second.Value.(string)
			} else if first.Type == P_BOOLEAN {
				result = first.Value.(bool) == second.Value.(bool)
			} else {
				panicf("[==] cannot compare types %v", first.Type)
			}
			globalStack = append(globalStack, PToken{P_BOOLEAN, result})
		},
		"!=": func() {
			second := globalStack.PopAny()
			first := globalStack.PopAny()
			var result bool
			if first.Type != second.Type {
				result = true
			} else if first.Type == P_INT {
				result = first.Value.(int64) != second.Value.(int64)
			} else if first.Type == P_FLOAT {
				result = first.Value.(float64) != second.Value.(float64)
			} else if first.Type == P_STRING {
				result = first.Value.(string) != second.Value.(string)
			} else if first.Type == P_BOOLEAN {
				result = first.Value.(bool) != second.Value.(bool)
			} else {
				panicf("[!=] cannot compare types %v", first.Type)
			}
			globalStack = append(globalStack, PToken{P_BOOLEAN, result})
		},
		"if": func() {
			block := globalStack.PopBlock()
			condition := globalStack.PopBoolean()
			if condition {
				run_function(block, nil)
			}
		},
		"loop": func() {
			block := globalStack.PopBlock()

			// Push loop context
			ctx := &loopContext{shouldBreak: false}
			loopStack = append(loopStack, ctx)
			defer func() {
				loopStack = loopStack[:len(loopStack)-1]
			}()

			// Infinite loop until break
			for {
				run_function(block, nil)

				// Check if break was called
				if ctx.shouldBreak {
					break
				}
			}
		},
		"break": func() {
			if len(loopStack) == 0 {
				panicf("[BREAK] called outside of loop")
			}
			// Set flag on innermost loop
			loopStack[len(loopStack)-1].shouldBreak = true
			// Panic to exit current iteration
			panic("BREAK")
		},
		"len": func() {
			item := globalStack.PopAny()
			var length int64
			if item.Type == P_STACK {
				stack := item.Value.(IStack)
				length = int64(len(stack))
			} else if item.Type == P_MEMORY {
				memory := item.Value.(IMemory)
				length = int64(len(memory))
			} else if item.Type == P_STRING {
				str := item.Value.(string)
				length = int64(len(str))
			} else {
				panicf("[LEN] cannot get length of type %v", item.Type)
			}
			// Push item back, then length
			globalStack = append(globalStack, item)
			globalStack = append(globalStack, PToken{P_INT, length})
		},
		"runfrom": func() {
			// Syntax: mem codeblock runfrom
			code_block_token := globalStack.PopBlock()
			mem_or_sym := globalStack.PopAny()

			var memory IMemory
			if mem_or_sym.Type == P_SYMBOL {
				// Load from scope: mymem { ... } runfrom
				varname := mem_or_sym.Value.(string)
				mem_token, ok := globalScope[varname]
				if !ok {
					panicf("[RUNFROM] variable %v not found", varname)
				}
				assert(mem_token.Type == P_MEMORY, "[RUNFROM] %v is not a memory, got %v", varname, mem_token.Type)
				memory = mem_token.Value.(IMemory)
			} else if mem_or_sym.Type == P_MEMORY {
				// Direct memory: [] { ... } runfrom
				memory = mem_or_sym.Value.(IMemory)
			} else {
				panicf("[RUNFROM] expected symbol or memory, got %v", mem_or_sym.Type)
			}

			// Convert IMemory to IScope (both are map[string]PToken)
			local_memory := IScope(memory)

			// Run code with local memory
			run_function(code_block_token, &local_memory)
		},
	}
}

func parser(code string, interp_chan chan PToken, wg *sync.WaitGroup, parsed_code_collect *IStack) {
	defer wg.Done()
	var current_state = PS_PARSING
	var word []rune
	// how deep is the parser
	var block_deepness = 0
	var stack_deepness = 0
	var memory_deepness = 0
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
		memory_deepness = 0
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
			} else if char == '[' {
				if len(word) > 0 {
					parse_word()
				}
				current_state = PS_MEMORY
				memory_deepness = 1
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
				if len(word) > 0 && word[len(word)-1] == '\\' {
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
		} else if current_state == PS_MEMORY {
			if char == ']' {
				if memory_deepness > 1 {
					memory_deepness -= 1
					word = append(word, char)
				} else {
					// build a token
					// send token
					// For now, only support empty memory []
					empty_memory := make(IMemory)
					push_value(empty_memory, P_MEMORY)
					// clear word
					reset_all()
				}
			} else {
				if char == '[' {
					memory_deepness += 1
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
		} else if memory_deepness > 0 {
			panic("[PRSR]: Memory never closed, might be a missing ']'")
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

func interpret(interp_chan chan PToken, wg *sync.WaitGroup, local_memory *IScope) {
	defer wg.Done()

	// Catch break panics
	defer func() {
		if r := recover(); r != nil {
			if r == "BREAK" {
				// Drain remaining tokens so parser can finish
				for range interp_chan {
				}
				return // Clean exit for break
			}
			panic(r) // Re-throw other panics
		}
	}()

	for token := range interp_chan {
		if token.Type == P_SYMBOL {
			symbol_name := token.Value.(string)

			// 1. Check if it's a builtin operation
			if builtin, ok := builtins[symbol_name]; ok {
				builtin()
				continue
				// 2. Check local memory (if exists)
			} else if local_memory != nil {
				if val, ok := (*local_memory)[symbol_name]; ok {
					globalStack = append(globalStack, val)
					continue
				}
			}
			// 3. Unknown symbol - push to stack
			globalStack = append(globalStack, token)
		} else {
			// Push literals to stack
			globalStack = append(globalStack, token)
		}
	}
}

func run_function(code_block string, local_memory *IScope) {
	interp_chan := make(chan PToken)
	var wg sync.WaitGroup
	wg.Add(2)
	go parser(code_block, interp_chan, &wg, nil)
	go interpret(interp_chan, &wg, local_memory)
	wg.Wait()
}

func main() {
	code_raw, _ := os.ReadFile("./test.nm")
	code := string(code_raw)

	interp_chan := make(chan PToken)
	var wg sync.WaitGroup
	wg.Add(2)
	go parser(code, interp_chan, &wg, nil)
	go interpret(interp_chan, &wg, nil)
	wg.Wait()
}
