//go:build generate
// +build generate

//go:generate go run gen.go -out galois_gen_amd64.s -stubs galois_gen_amd64.go
//go:generate gofmt -w galois_gen_switch_amd64.go

package main

import (
	"bufio"
	"fmt"
	"os"

	. "github.com/mmcloughlin/avo/build"
	"github.com/mmcloughlin/avo/buildtags"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
)

// Technically we can do slightly bigger, but we stay reasonable.
const inputMax = 10
const outputMax = 8

var switchDefs [inputMax][outputMax]string
var switchDefsX [inputMax][outputMax]string

const perLoopBits = 5
const perLoop = 1 << perLoopBits

func main() {
	Constraint(buildtags.Not("appengine").ToConstraint())
	Constraint(buildtags.Not("noasm").ToConstraint())
	Constraint(buildtags.Not("nogen").ToConstraint())
	Constraint(buildtags.Term("gc").ToConstraint())

	for i := 1; i <= inputMax; i++ {
		for j := 1; j <= outputMax; j++ {
			//genMulAvx2(fmt.Sprintf("mulAvxTwoXor_%dx%d", i, j), i, j, true)
			genMulAvx2(fmt.Sprintf("mulAvxTwo_%dx%d", i, j), i, j, false)
		}
	}
	f, err := os.Create("galois_gen_switch_amd64.go")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	w.WriteString(`// Code generated by command: go generate ` + os.Getenv("GOFILE") + `. DO NOT EDIT.

// +build !appengine
// +build !noasm
// +build gc
// +build !nogen 

package reedsolomon

import "fmt"

`)

	w.WriteString("const avx2CodeGen = true\n")
	w.WriteString(fmt.Sprintf("const maxAvx2Inputs = %d\nconst maxAvx2Outputs = %d\n", inputMax, outputMax))
	w.WriteString(`

func galMulSlicesAvx2(matrix []byte, in, out [][]byte, start, stop int) int {
	n := stop-start
`)

	w.WriteString(fmt.Sprintf("n = (n>>%d)<<%d\n\n", perLoopBits, perLoopBits))
	w.WriteString(`switch len(in) {
`)
	for in, defs := range switchDefs[:] {
		w.WriteString(fmt.Sprintf("		case %d:\n			switch len(out) {\n", in+1))
		for out, def := range defs[:] {
			w.WriteString(fmt.Sprintf("				case %d:\n", out+1))
			w.WriteString(def)
		}
		w.WriteString("}\n")
	}
	w.WriteString(`}
	panic(fmt.Sprintf("unhandled size: %dx%d", len(in), len(out)))
}
`)
	Generate()
}

func genMulAvx2(name string, inputs int, outputs int, xor bool) {
	total := inputs * outputs

	doc := []string{
		fmt.Sprintf("%s takes %d inputs and produces %d outputs.", name, inputs, outputs),
	}
	if !xor {
		doc = append(doc, "The output is initialized to 0.")
	}

	// Load shuffle masks on every use.
	var loadNone bool
	// Use registers for destination registers.
	var regDst = true

	// lo, hi, 1 in, 1 out, 2 tmp, 1 mask
	est := total*2 + outputs + 5
	if outputs == 1 {
		// We don't need to keep a copy of the input if only 1 output.
		est -= 2
	}

	if est > 16 {
		loadNone = true
		// We run out of GP registers first, now.
		if inputs+outputs > 12 {
			regDst = false
		}
	}

	TEXT(name, 0, fmt.Sprintf("func(matrix []byte, in [][]byte, out [][]byte, start, n int)"))

	// SWITCH DEFINITION:
	s := fmt.Sprintf("			mulAvxTwo_%dx%d(matrix, in, out, start, n)\n", inputs, outputs)
	s += fmt.Sprintf("\t\t\t\treturn n\n")
	switchDefs[inputs-1][outputs-1] = s

	if loadNone {
		Comment("Loading no tables to registers")
	} else {
		// loadNone == false
		Comment("Loading all tables to registers")
	}

	Doc(doc...)
	Pragma("noescape")
	Commentf("Full registers estimated %d YMM used", est)

	length := Load(Param("n"), GP64())
	matrixBase := GP64()
	MOVQ(Param("matrix").Base().MustAddr(), matrixBase)
	SHRQ(U8(perLoopBits), length)
	TESTQ(length, length)
	JZ(LabelRef(name + "_end"))

	dst := make([]reg.VecVirtual, outputs)
	dstPtr := make([]reg.GPVirtual, outputs)
	outBase := Param("out").Base().MustAddr()
	outSlicePtr := GP64()
	MOVQ(outBase, outSlicePtr)
	for i := range dst {
		dst[i] = YMM()
		if !regDst {
			continue
		}
		ptr := GP64()
		MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
		dstPtr[i] = ptr
	}

	inLo := make([]reg.VecVirtual, total)
	inHi := make([]reg.VecVirtual, total)

	for i := range inLo {
		if loadNone {
			break
		}
		tableLo := YMM()
		tableHi := YMM()
		VMOVDQU(Mem{Base: matrixBase, Disp: i * 64}, tableLo)
		VMOVDQU(Mem{Base: matrixBase, Disp: i*64 + 32}, tableHi)
		inLo[i] = tableLo
		inHi[i] = tableHi
	}

	inPtrs := make([]reg.GPVirtual, inputs)
	inSlicePtr := GP64()
	MOVQ(Param("in").Base().MustAddr(), inSlicePtr)
	for i := range inPtrs {
		ptr := GP64()
		MOVQ(Mem{Base: inSlicePtr, Disp: i * 24}, ptr)
		inPtrs[i] = ptr
	}

	tmpMask := GP64()
	MOVQ(U32(15), tmpMask)
	lowMask := YMM()
	MOVQ(tmpMask, lowMask.AsX())
	VPBROADCASTB(lowMask.AsX(), lowMask)

	offset := GP64()
	MOVQ(Param("start").MustAddr(), offset)
	Label(name + "_loop")
	if xor {
		Commentf("Load %d outputs", outputs)
	} else {
		Commentf("Clear %d outputs", outputs)
	}
	for i := range dst {
		if xor {
			if regDst {
				VMOVDQU(Mem{Base: dstPtr[i], Index: offset, Scale: 1}, dst[i])
				continue
			}
			ptr := GP64()
			MOVQ(outBase, ptr)
			VMOVDQU(Mem{Base: ptr, Index: offset, Scale: 1}, dst[i])
		} else {
			VPXOR(dst[i], dst[i], dst[i])
		}
	}

	lookLow, lookHigh := YMM(), YMM()
	inLow, inHigh := YMM(), YMM()
	for i := range inPtrs {
		Commentf("Load and process 32 bytes from input %d to %d outputs", i, outputs)
		VMOVDQU(Mem{Base: inPtrs[i], Index: offset, Scale: 1}, inLow)
		VPSRLQ(U8(4), inLow, inHigh)
		VPAND(lowMask, inLow, inLow)
		VPAND(lowMask, inHigh, inHigh)
		for j := range dst {
			if loadNone {
				VMOVDQU(Mem{Base: matrixBase, Disp: 64 * (i*outputs + j)}, lookLow)
				VMOVDQU(Mem{Base: matrixBase, Disp: 32 + 64*(i*outputs+j)}, lookHigh)
				VPSHUFB(inLow, lookLow, lookLow)
				VPSHUFB(inHigh, lookHigh, lookHigh)
			} else {
				VPSHUFB(inLow, inLo[i*outputs+j], lookLow)
				VPSHUFB(inHigh, inHi[i*outputs+j], lookHigh)
			}
			VPXOR(lookLow, lookHigh, lookLow)
			VPXOR(lookLow, dst[j], dst[j])
		}
	}
	Commentf("Store %d outputs", outputs)
	for i := range dst {
		if regDst {
			VMOVDQU(dst[i], Mem{Base: dstPtr[i], Index: offset, Scale: 1})
			continue
		}
		ptr := GP64()
		MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
		VMOVDQU(dst[i], Mem{Base: ptr, Index: offset, Scale: 1})
	}
	Comment("Prepare for next loop")
	ADDQ(U8(perLoop), offset)
	DECQ(length)
	JNZ(LabelRef(name + "_loop"))
	VZEROUPPER()

	Label(name + "_end")
	RET()
}
