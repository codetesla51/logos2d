package main

import (
	"fmt"
	"os"

	"github.com/codetesla51/logos/interpreter"
)

// newVM creates the sandboxed Logos VM used by the game.
// We hold the raw *interpreter.Interpreter (not the logos.VM wrapper) because
// the ECS layer must call stored script closures via Eval.
func newVM() *interpreter.Interpreter {
	return interpreter.NewInterpreter(interpreter.SandboxConfig{
		AllowFileIO:  false,
		AllowNetwork: false,
		AllowShell:   false,
		AllowExit:    false,
	})
}

// loadScript reads main.lgs from the working directory, runs it, and calls
// the script's on_load hook.
func loadScript(vm *interpreter.Interpreter) {
	source, err := os.ReadFile("main.lgs")
	if err != nil {
		panic(err)
	}

	if err := vm.Run(string(source)); err != nil {
		panic(err)
	}

	if _, err := vm.Call("on_load"); err != nil {
		panic(err)
	}
}

// callScript invokes a Logos function, logging runtime errors instead of
// crashing the game loop.
func callScript(vm *interpreter.Interpreter, fn string, args ...interface{}) {
	if _, err := vm.Call(fn, args...); err != nil {
		fmt.Println(fn, "error:", err)
	}
}
