package types

// Code generated by go-ipld-prime gengo.  DO NOT EDIT.

import (
	ipld "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/mixins"
	"github.com/ipld/go-ipld-prime/schema"
)

type _Bytes struct{ x []byte }
type Bytes = *_Bytes
func (n Bytes) Bytes() []byte {
	return n.x
}
func (_Bytes__Prototype) FromBytes(v []byte) (Bytes, error) {
	n := _Bytes{v}
	return &n, nil
}
type _Bytes__Maybe struct {
	m schema.Maybe
	v Bytes
}
type MaybeBytes = *_Bytes__Maybe

func (m MaybeBytes) IsNull() bool {
	return m.m == schema.Maybe_Null
}
func (m MaybeBytes) IsAbsent() bool {
	return m.m == schema.Maybe_Absent
}
func (m MaybeBytes) Exists() bool {
	return m.m == schema.Maybe_Value
}
func (m MaybeBytes) AsNode() ipld.Node {
	switch m.m {
		case schema.Maybe_Absent:
			return ipld.Absent
		case schema.Maybe_Null:
			return ipld.Null
		case schema.Maybe_Value:
			return m.v
		default:
			panic("unreachable")
	}
}
func (m MaybeBytes) Must() Bytes {
	if !m.Exists() {
		panic("unbox of a maybe rejected")
	}
	return m.v
}
var _ ipld.Node = (Bytes)(&_Bytes{})
var _ schema.TypedNode = (Bytes)(&_Bytes{})
func (Bytes) ReprKind() ipld.ReprKind {
	return ipld.ReprKind_Bytes
}
func (Bytes) LookupByString(string) (ipld.Node, error) {
	return mixins.Bytes{"types.Bytes"}.LookupByString("")
}
func (Bytes) LookupByNode(ipld.Node) (ipld.Node, error) {
	return mixins.Bytes{"types.Bytes"}.LookupByNode(nil)
}
func (Bytes) LookupByIndex(idx int) (ipld.Node, error) {
	return mixins.Bytes{"types.Bytes"}.LookupByIndex(0)
}
func (Bytes) LookupBySegment(seg ipld.PathSegment) (ipld.Node, error) {
	return mixins.Bytes{"types.Bytes"}.LookupBySegment(seg)
}
func (Bytes) MapIterator() ipld.MapIterator {
	return nil
}
func (Bytes) ListIterator() ipld.ListIterator {
	return nil
}
func (Bytes) Length() int {
	return -1
}
func (Bytes) IsAbsent() bool {
	return false
}
func (Bytes) IsNull() bool {
	return false
}
func (Bytes) AsBool() (bool, error) {
	return mixins.Bytes{"types.Bytes"}.AsBool()
}
func (Bytes) AsInt() (int, error) {
	return mixins.Bytes{"types.Bytes"}.AsInt()
}
func (Bytes) AsFloat() (float64, error) {
	return mixins.Bytes{"types.Bytes"}.AsFloat()
}
func (Bytes) AsString() (string, error) {
	return mixins.Bytes{"types.Bytes"}.AsString()
}
func (n Bytes) AsBytes() ([]byte, error) {
	return n.x, nil
}
func (Bytes) AsLink() (ipld.Link, error) {
	return mixins.Bytes{"types.Bytes"}.AsLink()
}
func (Bytes) Prototype() ipld.NodePrototype {
	return _Bytes__Prototype{}
}
type _Bytes__Prototype struct{}

func (_Bytes__Prototype) NewBuilder() ipld.NodeBuilder {
	var nb _Bytes__Builder
	nb.Reset()
	return &nb
}
type _Bytes__Builder struct {
	_Bytes__Assembler
}
func (nb *_Bytes__Builder) Build() ipld.Node {
	if *nb.m != schema.Maybe_Value {
		panic("invalid state: cannot call Build on an assembler that's not finished")
	}
	return nb.w
}
func (nb *_Bytes__Builder) Reset() {
	var w _Bytes
	var m schema.Maybe
	*nb = _Bytes__Builder{_Bytes__Assembler{w: &w, m: &m}}
}
type _Bytes__Assembler struct {
	w *_Bytes
	m *schema.Maybe
}

func (na *_Bytes__Assembler) reset() {}
func (_Bytes__Assembler) BeginMap(sizeHint int) (ipld.MapAssembler, error) {
	return mixins.BytesAssembler{"types.Bytes"}.BeginMap(0)
}
func (_Bytes__Assembler) BeginList(sizeHint int) (ipld.ListAssembler, error) {
	return mixins.BytesAssembler{"types.Bytes"}.BeginList(0)
}
func (na *_Bytes__Assembler) AssignNull() error {
	switch *na.m {
	case allowNull:
		*na.m = schema.Maybe_Null
		return nil
	case schema.Maybe_Absent:
		return mixins.BytesAssembler{"types.Bytes"}.AssignNull()
	case schema.Maybe_Value, schema.Maybe_Null:
		panic("invalid state: cannot assign into assembler that's already finished")
	}
	panic("unreachable")
}
func (_Bytes__Assembler) AssignBool(bool) error {
	return mixins.BytesAssembler{"types.Bytes"}.AssignBool(false)
}
func (_Bytes__Assembler) AssignInt(int) error {
	return mixins.BytesAssembler{"types.Bytes"}.AssignInt(0)
}
func (_Bytes__Assembler) AssignFloat(float64) error {
	return mixins.BytesAssembler{"types.Bytes"}.AssignFloat(0)
}
func (_Bytes__Assembler) AssignString(string) error {
	return mixins.BytesAssembler{"types.Bytes"}.AssignString("")
}
func (na *_Bytes__Assembler) AssignBytes(v []byte) error {
	switch *na.m {
	case schema.Maybe_Value, schema.Maybe_Null:
		panic("invalid state: cannot assign into assembler that's already finished")
	}
	if na.w == nil {
		na.w = &_Bytes{}
	}
	na.w.x = v
	*na.m = schema.Maybe_Value
	return nil
}
func (_Bytes__Assembler) AssignLink(ipld.Link) error {
	return mixins.BytesAssembler{"types.Bytes"}.AssignLink(nil)
}
func (na *_Bytes__Assembler) AssignNode(v ipld.Node) error {
	if v.IsNull() {
		return na.AssignNull()
	}
	if v2, ok := v.(*_Bytes); ok {
		switch *na.m {
		case schema.Maybe_Value, schema.Maybe_Null:
			panic("invalid state: cannot assign into assembler that's already finished")
		}
		if na.w == nil {
			na.w = v2
			*na.m = schema.Maybe_Value
			return nil
		}
		*na.w = *v2
		*na.m = schema.Maybe_Value
		return nil
	}
	if v2, err := v.AsBytes(); err != nil {
		return err
	} else {
		return na.AssignBytes(v2)
	}
}
func (_Bytes__Assembler) Prototype() ipld.NodePrototype {
	return _Bytes__Prototype{}
}
func (Bytes) Type() schema.Type {
	return nil /*TODO:typelit*/
}
func (n Bytes) Representation() ipld.Node {
	return (*_Bytes__Repr)(n)
}
type _Bytes__Repr = _Bytes
var _ ipld.Node = &_Bytes__Repr{}
type _Bytes__ReprPrototype = _Bytes__Prototype
type _Bytes__ReprAssembler = _Bytes__Assembler
