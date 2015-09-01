package loader

import (
	"bytes"
	"debug/elf"
	"errors"
	"fmt"
	"io"
)

var machineMap = map[elf.Machine]string{
	elf.EM_386:         "x86",
	elf.EM_X86_64:      "x86_64",
	elf.EM_ARM:         "arm",
	elf.EM_MIPS:        "mipseb",
	elf.EM_MIPS_RS3_LE: "mipsel",
	elf.EM_PPC:         "ppc",
	elf.EM_PPC64:       "ppc64",
}

type ElfLoader struct {
	LoaderHeader
	file *elf.File
}

var elfMagic = []byte{0x7f, 0x45, 0x4c, 0x46}

func MatchElf(r io.ReaderAt) bool {
	return bytes.Equal(getMagic(r), elfMagic)
}

func NewElfLoader(r io.ReaderAt) (Loader, error) {
	file, err := elf.NewFile(r)
	if err != nil {
		return nil, err
	}
	var bits int
	switch file.Class {
	case elf.ELFCLASS32:
		bits = 32
	case elf.ELFCLASS64:
		bits = 64
	default:
		return nil, errors.New("Unknown ELF class.")
	}
	machineName, ok := machineMap[file.Machine]
	if !ok {
		return nil, fmt.Errorf("Unsupported machine: %s", file.Machine)
	}
	return &ElfLoader{
		LoaderHeader: LoaderHeader{
			arch:  machineName,
			bits:  bits,
			os:    "linux",
			entry: file.Entry,
		},
		file: file,
	}, nil
}

func (e *ElfLoader) DataSegment() (start, end uint64) {
	sec := e.file.Section(".data")
	if sec != nil {
		return sec.Addr, sec.Addr + sec.Size
	}
	return 0, 0
}

func (e *ElfLoader) Segments() ([]Segment, error) {
	ret := make([]Segment, 0, len(e.file.Progs))
	for _, prog := range e.file.Progs {
		if prog.Type != elf.PT_LOAD {
			continue
		}
		data := make([]byte, prog.Memsz)
		prog.Open().Read(data)
		ret = append(ret, Segment{
			Addr: prog.Vaddr,
			Data: data,
		})
	}
	return ret, nil
}

func (e *ElfLoader) Symbolicate(addr uint64) (string, error) {
	nearest := make(map[uint64][]elf.Symbol)
	syms, err := e.file.Symbols()
	if err != nil {
		return "", err
	}
	for _, sym := range syms {
		dist := addr - sym.Value
		if dist > 0 && dist <= sym.Size {
			nearest[dist] = append(nearest[dist], sym)
		}
	}
	if len(nearest) > 0 {
		for dist, v := range nearest {
			sym := v[0]
			return fmt.Sprintf("%s+0x%x", sym.Name, dist), nil
		}
	}
	return "", nil
}
