package main

import (
	"fmt"
)

type IStack []PToken

// PopAny pops the last item from the stack
func (s *IStack) PopAny() PToken {
	if len(*s) == 0 {
		panic("[POP] PopAny called on an empty stack")
	}

	lastIndex := len(*s) - 1
	token := (*s)[lastIndex]
	*s = (*s)[:lastIndex] // Remove the token from the stack

	return token
}

func (s *IStack) PopStack() IStack {
	if len(*s) == 0 {
		panic("[POP] PopStack called on an empty stack")
	}

	lastIndex := len(*s) - 1
	token := (*s)[lastIndex]
	*s = (*s)[:lastIndex] // Remove the token from the stack

	if token.Type != P_STACK {
		panic(fmt.Sprintf("[POP] type mismatch: expected P_STACK, got %d", token.Type))
	}
	if v, ok := token.Value.(IStack); ok {
		return v
	}

	panic("[POP] failed to cast value to Stack")
}

func (s *IStack) PopBlock() string {
	if len(*s) == 0 {
		panic("[POP] PopBlock called on an empty stack")
	}

	lastIndex := len(*s) - 1
	token := (*s)[lastIndex]
	*s = (*s)[:lastIndex] // Remove the token from the stack

	if token.Type != P_BLOCK {
		panic(fmt.Sprintf("[POP] type mismatch: expected P_BLOCK, got %d", token.Type))
	}
	if v, ok := token.Value.(string); ok {
		return v
	}

	panic("[POP] failed to cast value to Block/String")
}

// PopInt pops the last item from the stack, ensures it's an integer, and returns it.
func (s *IStack) PopInt() int64 {
	if len(*s) == 0 {
		panic("[POP] PopInt called on an empty stack")
	}

	lastIndex := len(*s) - 1
	token := (*s)[lastIndex]
	*s = (*s)[:lastIndex] // Remove the token from the stack

	if token.Type != P_INT {
		panic(fmt.Sprintf("[POP] type mismatch: expected P_INT, got %d", token.Type))
	}

	if v, ok := token.Value.(int64); ok {
		return v
	}

	panic("[POP] failed to cast value to int")
}

// PopFloat pops the last item from the stack, ensures it's a float64, and returns it.
func (s *IStack) PopFloat() float64 {
	if len(*s) == 0 {
		panic("[POP] PopFloat called on an empty stack")
	}

	lastIndex := len(*s) - 1
	token := (*s)[lastIndex]
	*s = (*s)[:lastIndex] // Remove the token from the stack

	if token.Type != P_FLOAT {
		panic(fmt.Sprintf("[POP] type mismatch: expected P_FLOAT, got %d", token.Type))
	}

	if v, ok := token.Value.(float64); ok {
		return v
	}

	panic("[POP] failed to cast value to float64")
}

// PopString pops the last item from the stack, ensures it's a string, and returns it.
func (s *IStack) PopString() string {
	if len(*s) == 0 {
		panic("[POP] PopString called on an empty stack")
	}

	lastIndex := len(*s) - 1
	token := (*s)[lastIndex]
	*s = (*s)[:lastIndex] // Remove the token from the stack

	if !Contains(token.Type, P_STRING, P_SYMBOL) {
		panic(fmt.Sprintf("[POP] type mismatch: expected P_STRING or P_SYMBOL, got %d", token.Type))
	}

	if v, ok := token.Value.(string); ok {
		return v
	}

	panic("[POP] failed to cast value to string")
}

// PopBoolean pops the last item from the stack, ensures it's a bool, and returns it.
func (s *IStack) PopBoolean() bool {
	if len(*s) == 0 {
		panic("[POP] PopBoolean called on an empty stack")
	}

	lastIndex := len(*s) - 1
	token := (*s)[lastIndex]
	*s = (*s)[:lastIndex] // Remove the token from the stack

	if token.Type != P_BOOLEAN {
		panic(fmt.Sprintf("[POP] type mismatch: expected P_BOOLEAN, got %d", token.Type))
	}

	if v, ok := token.Value.(bool); ok {
		return v
	}

	panic("[POP] failed to cast value to bool")
}

func Contains[T comparable](value T, slice ...T) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func assert(condition bool, errorText string, args ...any) {
	if !condition {
		panic(fmt.Sprintf(errorText, args...))
	}
}

func panicf(errorText string, args ...any) {
	panic(fmt.Sprintf(errorText, args...))
}

func (s *IStack) PushFront(elem PToken) {
	assert(s != nil, "IStack should not be nil")
	*s = append([]PToken{elem}, (*s)...)
}
