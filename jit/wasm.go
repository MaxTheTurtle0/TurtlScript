package jit

// These encodings use WebAssembly Core 2 instructions and multi-value results.
// Every language value is an i64. Static kinds distinguish integers, booleans,
// and null; a second i64 result carries a language error code.
const (
	opBlock       = 0x02
	opLoop        = 0x03
	opIf          = 0x04
	opElse        = 0x05
	opEnd         = 0x0b
	opBr          = 0x0c
	opBrIf        = 0x0d
	opReturn      = 0x0f
	opCall        = 0x10
	opDrop        = 0x1a
	opLocalGet    = 0x20
	opLocalSet    = 0x21
	opI32Const    = 0x41
	opI64Const    = 0x42
	opI32Eqz      = 0x45
	opI64Eqz      = 0x50
	opI64Eq       = 0x51
	opI64Ne       = 0x52
	opI64LtS      = 0x53
	opI64GtS      = 0x55
	opI64LeS      = 0x57
	opI64GeS      = 0x59
	opI64Add      = 0x7c
	opI64Sub      = 0x7d
	opI64Mul      = 0x7e
	opI64DivS     = 0x7f
	opI64RemS     = 0x81
	opI64And      = 0x83
	opI64Or       = 0x84
	opI64Xor      = 0x85
	opI64Shl      = 0x86
	opI64ShrS     = 0x87
	opI64ShrU     = 0x88
	opI64Extend32 = 0xad
	wasmI64       = 0x7e
	wasmEmpty     = 0x40
)

func unsigned(value uint32) []byte {
	var result []byte
	for {
		b := byte(value & 0x7f)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		result = append(result, b)
		if value == 0 {
			return result
		}
	}
}

func signed(value int64) []byte {
	var result []byte
	for {
		b := byte(value & 0x7f)
		value >>= 7
		last := value == 0 && b&0x40 == 0 || value == -1 && b&0x40 != 0
		if !last {
			b |= 0x80
		}
		result = append(result, b)
		if last {
			return result
		}
	}
}

func section(module []byte, id byte, data []byte) []byte {
	module = append(module, id)
	module = append(module, unsigned(uint32(len(data)))...)
	return append(module, data...)
}

func (g *generator) emit(instructions ...byte) { g.body = append(g.body, instructions...) }
func (g *generator) index(op byte, index uint32) {
	g.emit(op)
	g.body = append(g.body, unsigned(index)...)
}
func (g *generator) constant(value int64) {
	g.emit(opI64Const)
	g.body = append(g.body, signed(value)...)
}
func (g *generator) get(index uint32) { g.index(opLocalGet, index) }
func (g *generator) set(index uint32) { g.index(opLocalSet, index) }
func (g *generator) temporary() uint32 {
	index := g.parameters + g.locals
	g.locals++
	return index
}

func (g *generator) module() []byte {
	module := []byte{0, 'a', 's', 'm', 1, 0, 0, 0}
	typ := append([]byte{1, 0x60}, unsigned(g.parameters)...)
	for range g.parameters {
		typ = append(typ, wasmI64)
	}
	typ = append(typ, 2, wasmI64, wasmI64)
	module = section(module, 1, typ)
	module = section(module, 3, []byte{1, 0}) // One function of type zero.
	module = section(module, 7, []byte{1, 3, 'r', 'u', 'n', 0, 0})
	body := append([]byte{1}, unsigned(g.locals)...)
	body = append(body, wasmI64)
	body = append(body, g.body...)
	body = append(body, opEnd)
	functions := append([]byte{1}, unsigned(uint32(len(body)))...)
	module = section(module, 10, append(functions, body...))
	return module
}
