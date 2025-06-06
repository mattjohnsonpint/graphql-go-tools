package ast

import (
	"bytes"

	"github.com/wundergraph/graphql-go-tools/v2/pkg/internal/unsafebytes"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/lexer/position"
)

// UnionTypeDefinition
// example:
// union SearchResult = Photo | Person
type UnionTypeDefinition struct {
	Description         Description        // optional, describes union
	UnionLiteral        position.Position  // union
	Name                ByteSliceReference // e.g. SearchResult
	HasDirectives       bool
	Directives          DirectiveList     // optional, e.g. @foo
	Equals              position.Position // =
	HasUnionMemberTypes bool
	UnionMemberTypes    TypeList // optional, e.g. Photo | Person
	HasFieldDefinitions bool
	FieldsDefinition    FieldDefinitionList // contains a single field: { __typename: String! }
}

func (d *Document) UnionTypeDefinitionNameBytes(ref int) ByteSlice {
	return d.Input.ByteSlice(d.UnionTypeDefinitions[ref].Name)
}

func (d *Document) UnionTypeDefinitionNameString(ref int) string {
	return unsafebytes.BytesToString(d.Input.ByteSlice(d.UnionTypeDefinitions[ref].Name))
}

func (d *Document) UnionTypeDefinitionDescriptionBytes(ref int) ByteSlice {
	if !d.UnionTypeDefinitions[ref].Description.IsDefined {
		return nil
	}
	return d.Input.ByteSlice(d.UnionTypeDefinitions[ref].Description.Content)
}

func (d *Document) UnionTypeDefinitionDescriptionString(ref int) string {
	return unsafebytes.BytesToString(d.UnionTypeDefinitionDescriptionBytes(ref))
}

func (d *Document) UnionTypeDefinitionHasField(ref int, fieldName []byte) bool {
	for _, fieldRef := range d.UnionTypeDefinitions[ref].FieldsDefinition.Refs {
		if bytes.Equal(d.FieldDefinitionNameBytes(fieldRef), fieldName) {
			return true
		}
	}
	return false
}

func (d *Document) UnionMemberTypeIsFirst(ref int, ancestor Node) bool {
	switch ancestor.Kind {
	case NodeKindUnionTypeDefinition:
		return len(d.UnionTypeDefinitions[ancestor.Ref].UnionMemberTypes.Refs) != 0 &&
			d.UnionTypeDefinitions[ancestor.Ref].UnionMemberTypes.Refs[0] == ref
	case NodeKindUnionTypeExtension:
		return len(d.UnionTypeExtensions[ancestor.Ref].UnionMemberTypes.Refs) != 0 &&
			d.UnionTypeExtensions[ancestor.Ref].UnionMemberTypes.Refs[0] == ref
	default:
		return false
	}
}

func (d *Document) UnionMemberTypeIsLast(ref int, ancestor Node) bool {
	switch ancestor.Kind {
	case NodeKindUnionTypeDefinition:
		return len(d.UnionTypeDefinitions[ancestor.Ref].UnionMemberTypes.Refs) != 0 &&
			d.UnionTypeDefinitions[ancestor.Ref].UnionMemberTypes.Refs[len(d.UnionTypeDefinitions[ancestor.Ref].UnionMemberTypes.Refs)-1] == ref
	case NodeKindUnionTypeExtension:
		return len(d.UnionTypeExtensions[ancestor.Ref].UnionMemberTypes.Refs) != 0 &&
			d.UnionTypeExtensions[ancestor.Ref].UnionMemberTypes.Refs[len(d.UnionTypeExtensions[ancestor.Ref].UnionMemberTypes.Refs)-1] == ref
	default:
		return false
	}
}

func (d *Document) UnionTypeDefinitionHasDirectives(ref int) bool {
	return d.UnionTypeDefinitions[ref].HasDirectives
}

func (d *Document) AddUnionTypeDefinition(definition UnionTypeDefinition) (ref int) {
	d.UnionTypeDefinitions = append(d.UnionTypeDefinitions, definition)
	return len(d.UnionTypeDefinitions) - 1
}

func (d *Document) ImportUnionTypeDefinition(name, description string, typeRefs []int) (ref int) {
	return d.ImportUnionTypeDefinitionWithDirectives(name, description, typeRefs, nil)
}

func (d *Document) ImportUnionTypeDefinitionWithDirectives(name, description string, typeRefs []int, directiveRefs []int) (ref int) {
	definition := UnionTypeDefinition{
		Name:                d.Input.AppendInputString(name),
		Description:         d.ImportDescription(description),
		HasUnionMemberTypes: len(typeRefs) > 0,
		UnionMemberTypes: TypeList{
			Refs: typeRefs,
		},
		HasDirectives: len(directiveRefs) > 0,
		Directives: DirectiveList{
			Refs: directiveRefs,
		},
	}

	ref = d.AddUnionTypeDefinition(definition)
	d.ImportRootNode(ref, NodeKindUnionTypeDefinition)

	return
}

func (d *Document) UnionTypeDefinitionMemberTypeNames(ref int) (typeNames []string, ok bool) {
	if !d.UnionTypeDefinitions[ref].HasUnionMemberTypes {
		return nil, false
	}

	typeNames = make([]string, 0, len(d.UnionTypeDefinitions[ref].UnionMemberTypes.Refs))
	for _, typeRef := range d.UnionTypeDefinitions[ref].UnionMemberTypes.Refs {
		typeNames = append(typeNames, d.TypeNameString(typeRef))
	}
	return typeNames, true
}

func (d *Document) UnionTypeDefinitionMemberTypeNamesAsBytes(ref int) (typeNames [][]byte, ok bool) {
	if !d.UnionTypeDefinitions[ref].HasUnionMemberTypes {
		return nil, false
	}

	typeNames = make([][]byte, 0, len(d.UnionTypeDefinitions[ref].UnionMemberTypes.Refs))
	for _, typeRef := range d.UnionTypeDefinitions[ref].UnionMemberTypes.Refs {
		typeNames = append(typeNames, d.TypeNameBytes(typeRef))
	}
	return typeNames, true
}

func (d *Document) UnionHasMember(ref int, typeName ByteSlice) bool {
	for _, i := range d.UnionTypeDefinitions[ref].UnionMemberTypes.Refs {
		memberName := d.ResolveTypeNameBytes(i)
		if bytes.Equal(typeName, memberName) {
			return true
		}
	}
	return false
}
