package expression_processing

import (
	"fmt"
	"strings"
	"text/scanner"
)

type Symbol struct {
	Val string
}

type Node struct {
	NodeId     string `json:"nodeId"`
	Op         string `json:"op"`
	X          *Node  `json:"x"`
	Y          *Node  `json:"y"`
	Val        string `json:"Val"`
	Sheet      bool   `json:"sheet"`
	Calculated bool   `json:"calculated"`
	Err        error  `json:"err"`
}

var priority = map[string]int{
	"+": 1,
	"-": 1,
	"*": 2,
	"/": 2,
	"^": 3,
}

type additiveStack interface {
	Node | Symbol
}

type Stack[T additiveStack] struct {
	val []*T
}

func (s *Stack[T]) push(l *T) {
	s.val = append(s.val, l)
}

func (s *Stack[T]) pop() *T {
	length := len(s.val)
	if !s.isEmpty() {
		res := s.val[length-1]
		s.val = s.val[:length-1]
		return res
	} else {
		return nil
	}
}

func (s *Stack[T]) isEmpty() bool {
	return len(s.val) == 0
}

func (s *Stack[T]) get() *T {
	if s.isEmpty() {
		return nil
	}
	return s.val[len(s.val)-1]
}

func (s *Symbol) getPriority() int {
	switch s.Val {
	case "+", "-", "*", "/", "^":
		return priority[s.Val]
	case "(", ")":
		return 0
	default:
		return 10
	}
}

func Parse(input string) ([]*Symbol, error) {
	var s scanner.Scanner
	s.Init(strings.NewReader(input))
	s.Mode = scanner.ScanFloats | scanner.ScanInts
	var SymbList []*Symbol
	for token := s.Scan(); token != scanner.EOF; token = s.Scan() {
		text := s.TokenText()
		switch token {
		case scanner.Int, scanner.Float:
			SymbList = append(SymbList, &Symbol{text})
		default:
			switch text {
			case "+", "-", "*", "/", "(", ")", "^":
				SymbList = append(SymbList, &Symbol{text})
			default:
				return nil, fmt.Errorf("invalid expression: %s", text)
			}
		}
	}
	postFix, err := getPostfix(SymbList)
	if err != nil {
		return nil, err
	}
	return postFix, nil
}

func (s *Symbol) getType() string {
	switch s.Val {
	case "+", "-", "*", "/", "(", ")", "^":
		return "Op"
	default:
		return "num"
	}
}
func getPostfix(input []*Symbol) ([]*Symbol, error) {
	var opStack Stack[Symbol]
	var postFix []*Symbol

	for _, currentSymbol := range input {
		switch currentSymbol.getType() {
		case "num":
			postFix = append(postFix, &Symbol{Val: currentSymbol.Val})
		case "Op":
			switch currentSymbol.Val {
			case "(":
				opStack.push(currentSymbol)
			case ")":
				for {
					headStack := opStack.pop()
					if headStack == nil {
						return nil, fmt.Errorf("invalid paranthesis")
					}
					if headStack.Val != "(" {
						postFix = append(postFix, headStack)
					} else {
						break
					}
				}
			default:
				priorCur := currentSymbol.getPriority()
				for !opStack.isEmpty() && opStack.get().getPriority() >= priorCur {
					postFix = append(postFix, opStack.pop())
				}
				opStack.push(currentSymbol)
			}
		}
	}
	for !opStack.isEmpty() {
		postFix = append(postFix, opStack.pop())
	}
	return postFix, nil
}

func PostfixToString(postfix []*Symbol) string {
	var result strings.Builder

	for _, symbol := range postfix {
		result.WriteString(symbol.Val)
		result.WriteString(" ")
	}

	return result.String()
}
