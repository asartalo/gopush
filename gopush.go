// Package gopush provides an implementation of Push 3.0, a stack-based
// programming language designed for genetic programming
package gopush

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

// Interpreter is a Push interpreter.
type Interpreter struct {
	Stacks      map[string]*Stack
	Options     Options
	Rand        *rand.Rand
	Definitions map[string]Code

	numEvalPush   int
	quoteNextName bool
}

// NewInterpreter returns a new Push Interpreter, configured with the provided Options.
func NewInterpreter(options Options) *Interpreter {
	interpreter := &Interpreter{
		Stacks:        make(map[string]*Stack),
		Options:       options,
		Rand:          rand.New(rand.NewSource(options.RandomSeed)),
		Definitions:   make(map[string]Code),
		numEvalPush:   0,
		quoteNextName: false,
	}

	interpreter.Stacks["integer"] = newIntStack(interpreter)
	interpreter.Stacks["float"] = newFloatStack(interpreter)
	interpreter.Stacks["exec"] = newExecStack(interpreter)
	interpreter.Stacks["code"] = newCodeStack(interpreter)
	interpreter.Stacks["name"] = newNameStack(interpreter)
	interpreter.Stacks["boolean"] = newBooleanStack(interpreter)

	return interpreter
}

func (i *Interpreter) stackOK(name string, mindepth int64) bool {
	s, ok := i.Stacks[name]
	if !ok {
		return false
	}

	if s.Len() < mindepth {
		return false
	}

	return true
}

func (i *Interpreter) printInterpreterState() {
	fmt.Println("Step", i.numEvalPush)
	for k, v := range i.Stacks {
		fmt.Printf("%s:\n", k)
		for i := len(v.Stack) - 1; i >= 0; i-- {
			fmt.Printf("- %v\n", v.Stack[i])
		}
	}
	fmt.Println()
	fmt.Println()
}

func (i *Interpreter) runCode(program Code) (err error) {

	// Recover from a panic that could occur while executing an instruction.
	// Because it is more convenient for functions to not return an error,
	// the functions that want to return an error panic instead.
	defer func() {
		if perr := recover(); perr != nil {
			err = perr.(error)
		}
	}()

	i.Stacks["exec"].Push(program)

	for i.Stacks["exec"].Len() > 0 && i.numEvalPush < i.Options.EvalPushLimit {

		if i.Options.Tracing {
			i.printInterpreterState()
		}

		item := i.Stacks["exec"].Pop().(Code)
		i.numEvalPush++

		// If the item on top of the exec stack is a list, push it in
		// reverse order
		if item.Literal == "" {
			for j := len(item.List) - 1; j >= 0; j-- {
				i.Stacks["exec"].Push(item.List[j])
			}
			continue
		}

		// Try to parse the item on top of the exec stack as a literal
		if intlit, err := strconv.ParseInt(item.Literal, 10, 64); err == nil {
			i.Stacks["integer"].Push(intlit)
			continue
		}

		if floatlit, err := strconv.ParseFloat(item.Literal, 64); err == nil {
			i.Stacks["float"].Push(floatlit)
			continue
		}

		if boollit, err := strconv.ParseBool(item.Literal); err == nil {
			i.Stacks["boolean"].Push(boollit)
			continue
		}

		// Try to parse the item on top of the exec stack as instruction
		if strings.Contains(item.Literal, ".") {
			stack := strings.ToLower(item.Literal[:strings.Index(item.Literal, ".")])
			operation := strings.ToLower(item.Literal[strings.Index(item.Literal, ".")+1:])

			s, ok := i.Stacks[stack]
			if !ok {
				return fmt.Errorf("unknown or disabled stack: %v", stack)
			}

			f, ok := s.Functions[operation]
			if !ok {
				return fmt.Errorf("unknown or disabled instruction %v.%v", stack, operation)
			}

			f()
			continue
		}

		// If the item is not an instruction, it must be a name, either
		// bound or unbound. If the quoteNextName flag is false, we can
		// check if the name is already bound.
		if !i.quoteNextName {
			if d, ok := i.Definitions[strings.ToLower(item.Literal)]; ok {
				// Name is already bound, push its value onto the exec stack
				i.Stacks["exec"].Push(d)
				continue
			}
		}

		// The name is not bound yet, so push it onto the name stack
		i.Stacks["name"].Push(strings.ToLower(item.Literal))
		i.quoteNextName = false
	}

	if i.numEvalPush >= i.Options.EvalPushLimit {
		return errors.New("the EvalPushLimit was exceeded")
	}

	return nil
}

// Run runs the given program written in the Push programming language until the
// EvalPushLimit is reached
func (i *Interpreter) Run(program string) error {
	c, err := ParseCode(program)
	if err != nil {
		return err
	}

	if i.Options.TopLevelPushCode {
		i.Stacks["code"].Push(c)
	}

	err = i.runCode(c)

	if i.Options.TopLevelPopCode {
		i.Stacks["code"].Pop()
	}

	if i.Options.Tracing {
		i.printInterpreterState()
	}

	return err
}
