package mips32

import (
	"errors"
	"strconv"
)

type Executable struct {
	// Segments maps chunks of instructions to various parts of the address space.
	Segments map[uint32][]Instruction

	// Symbols maps symbol names to their addresses.
	Symbols map[string]uint32
}

// ParseExecutable turns a tokenized source file into an executable blob.
//
// If the executable cannot be parsed for any reason, this will fail.
// Overlapping .text sections, invalid instructions, and repeated symbols will all cause errors.
func ParseExecutable(lines []TokenizedLine) (*Executable, error) {
	var segmentStart uint32
	var instructionAddr uint32
	res := &Executable{
		Segments: map[uint32][]Instruction{},
		Symbols:  map[string]uint32{},
	}
	for _, line := range lines {
		if line.Instruction != nil {
			parsed, err := ParseTokenizedInstruction(line.Instruction)
			if err != nil {
				return nil, errors.New("line " + strconv.Itoa(line.LineNumber) + ": " + err.Error())
			}
			if res.addressInUse(instructionAddr) {
				return nil, addressInUseError(line.LineNumber, instructionAddr)
			}
			res.Segments[segmentStart] = append(res.Segments[segmentStart], *parsed)
			instructionAddr += 4
		} else if line.Directive != nil {
			dir := line.Directive
			if dir.Name == ".word" {
				if res.addressInUse(instructionAddr) {
					return nil, addressInUseError(line.LineNumber, instructionAddr)
				}
				nextInst := DecodeInstruction(dir.Constant)
				res.Segments[segmentStart] = append(res.Segments[segmentStart], *nextInst)
				instructionAddr += 4
			} else if dir.Name == ".text" {
				if dir.Constant&3 != 0 {
					return nil, errors.New("line " + strconv.Itoa(line.LineNumber) +
						": misaligned segment")
				}
				segmentStart = dir.Constant
				instructionAddr = dir.Constant
			} else {
				return nil, errors.New("line " + strconv.Itoa(line.LineNumber) +
					": unknown directive: " + dir.Name)
			}
		} else if line.SymbolMarker != nil {
			sym := *line.SymbolMarker
			if _, ok := res.Symbols[sym]; ok {
				return nil, errors.New("line " + strconv.Itoa(line.LineNumber) +
					": repeated symbol declaration: " + sym)
			}
			res.Symbols[sym] = instructionAddr
		}
	}
	return res, nil
}

// addressInUse reports of a word-aligned address is being used by one of the segments.
func (e *Executable) addressInUse(addr uint32) bool {
	for segment, insts := range e.Segments {
		if segment <= addr && segment+uint32(len(insts)*4) > addr {
			return true
		}
	}
	return false
}

// Instruction stores all of the information about an instruction in distinct fields.
// This makes it possible to execute an instruction and see exactly what its operands are.
type Instruction struct {
	// Name is the instruction's name.
	//
	// If this instruction is from a ".word" directive which does not correspond to a valid
	// instruction, then the Name field is ".word" and the RawWord field is set.
	Name string

	// Registers is the list of register indices passed to this instruction.
	// This list is in the same order as the instruction's operands in assembly.
	Registers []int

	UnsignedConstant16 uint16
	SignedConstant16   int16
	Constant5          uint8
	CodePointer        CodePointer
	MemoryReference    MemoryReference

	// RawWord is only used for instructions which cannot be decoded.
	// This is only used when Name is set to ".word"
	RawWord uint32
}

// ParseTokenizedInstruction generates an Instruction which represents a TokenizedInstruction.
// This may fail if the instruction is invalid, in which case an error is returned.
func ParseTokenizedInstruction(t *TokenizedInstruction) (*Instruction, error) {
	validName := false
	for _, template := range Templates {
		if template.Name == t.Name {
			validName = true
		}
		if template.Match(t) {
			res := &Instruction{Name: t.Name}
			for i, arg := range template.Arguments {
				tokArg := t.Arguments[i]
				switch arg {
				case Register:
					reg, _ := tokArg.Register()
					res.Registers = append(res.Registers, reg)
				case SignedConstant16:
					res.SignedConstant16, _ = tokArg.SignedConstant16()
				case UnsignedConstant16:
					res.UnsignedConstant16, _ = tokArg.UnsignedConstant16()
				case Constant5:
					res.Constant5, _ = tokArg.Constant5()
				case AbsoluteCodePointer:
					res.CodePointer, _ = tokArg.AbsoluteCodePointer()
				case RelativeCodePointer:
					res.CodePointer, _ = tokArg.RelativeCodePointer()
				case MemoryAddress:
					res.MemoryReference, _ = tokArg.MemoryReference()
				}
			}
			return res, nil
		}
	}
	if validName {
		return nil, errors.New("bad instruction usage for " + t.Name)
	} else {
		return nil, errors.New("unknown instruction: " + t.Name)
	}
}

func addressInUseError(line int, addr uint32) error {
	hexStr := "0x" + strconv.FormatUint(uint64(addr), 16)
	return errors.New("line " + strconv.Itoa(line) + ": overwriting address " + hexStr)
}
