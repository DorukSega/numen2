package main

import (
	"fmt"
)

type IStack []PToken

// PopInt pops the last item from the stack, ensures it's an integer, and returns it.
func (s *IStack) PopAny() PToken {
	if len(*s) == 0 {
		panic("[POP] PopAny called on an empty stack")
	}

	lastIndex := len(*s) - 1
	token := (*s)[lastIndex]
	*s = (*s)[:lastIndex] // Remove the token from the stack

	return token
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

	if token.Type != P_STRING {
		panic(fmt.Sprintf("[POP] type mismatch: expected P_STRING, got %d", token.Type))
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

func Contains[T comparable](value T, slice []T) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
