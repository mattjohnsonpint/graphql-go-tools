package introspection

import (
	"strings"

	"github.com/wundergraph/graphql-go-tools/v2/pkg/ast"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/astvisitor"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/internal/unsafebytes"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/operationreport"
)

const (
	DeprecatedDirectiveName  = "deprecated"
	DeprecationReasonArgName = "reason"
	SpecifiedByDirectiveName = "specifiedBy"
)

type Generator struct {
	Data    *Data
	walker  *astvisitor.Walker
	visitor *introspectionVisitor
}

func NewGenerator() *Generator {
	walker := astvisitor.NewWalker(48)
	visitor := introspectionVisitor{
		Walker: &walker,
	}

	walker.RegisterDocumentVisitor(&visitor)
	walker.RegisterEnterDirectiveLocationVisitor(&visitor)
	walker.RegisterEnterInputValueDefinitionVisitor(&visitor)
	walker.RegisterEnterRootOperationTypeDefinitionVisitor(&visitor)
	walker.RegisterEnterScalarTypeDefinitionVisitor(&visitor)
	walker.RegisterEnterUnionMemberTypeVisitor(&visitor)
	walker.RegisterEnterSchemaDefinitionVisitor(&visitor)

	walker.RegisterDirectiveDefinitionVisitor(&visitor)
	walker.RegisterEnumTypeDefinitionVisitor(&visitor)
	walker.RegisterFieldDefinitionVisitor(&visitor)
	walker.RegisterInputObjectTypeDefinitionVisitor(&visitor)
	walker.RegisterInterfaceTypeDefinitionVisitor(&visitor)
	walker.RegisterObjectTypeDefinitionVisitor(&visitor)
	walker.RegisterUnionTypeDefinitionVisitor(&visitor)

	walker.RegisterLeaveEnumValueDefinitionVisitor(&visitor)

	return &Generator{
		walker:  &walker,
		visitor: &visitor,
	}
}

func (g *Generator) Generate(definition *ast.Document, report *operationreport.Report, data *Data) {
	g.visitor.data = data
	g.visitor.definition = definition
	g.walker.Walk(definition, nil, report)
}

type introspectionVisitor struct {
	*astvisitor.Walker
	definition       *ast.Document
	data             *Data
	currentType      *FullType
	currentField     Field
	currentDirective Directive

	queryTypeName        string
	mutationTypeName     string
	subscriptionTypeName string
}

func (i *introspectionVisitor) EnterDocument(operation, definition *ast.Document) {
	i.data.Schema = NewSchema()
}

func (i *introspectionVisitor) LeaveDocument(operation, definition *ast.Document) {
	if i.queryTypeName != "" {
		i.data.Schema.QueryType = *i.data.Schema.TypeByName(i.queryTypeName)
	}

	if i.mutationTypeName != "" {
		i.data.Schema.MutationType = i.data.Schema.TypeByName(i.mutationTypeName)
	}

	if i.subscriptionTypeName != "" {
		i.data.Schema.SubscriptionType = i.data.Schema.TypeByName(i.subscriptionTypeName)
	}
}

func (i *introspectionVisitor) EnterSchemaDefinition(ref int) {
	if !i.definition.SchemaDefinitions[ref].Description.IsDefined {
		return
	}

	description := unsafebytes.BytesToString(i.definition.Input.ByteSlice(i.definition.SchemaDefinitions[ref].Description.Content))
	i.data.Schema.Description = &description
}

func (i *introspectionVisitor) EnterObjectTypeDefinition(ref int) {
	i.currentType = NewFullType()
	i.currentType.Name = i.definition.ObjectTypeDefinitionNameString(ref)
	i.currentType.Kind = OBJECT
	i.currentType.Description = i.definition.ObjectTypeDescriptionNameString(ref)
	for _, typeRef := range i.definition.ObjectTypeDefinitions[ref].ImplementsInterfaces.Refs {
		name := i.definition.TypeNameString(typeRef)
		i.currentType.Interfaces = append(i.currentType.Interfaces, TypeRef{
			Kind:     INTERFACE,
			Name:     &name,
			TypeName: "__Type",
		})
	}
}

func (i *introspectionVisitor) LeaveObjectTypeDefinition(ref int) {
	if strings.HasPrefix(i.currentType.Name, "__") {
		return
	}
	i.data.Schema.AddType(i.currentType)
}

func (i *introspectionVisitor) EnterFieldDefinition(ref int) {
	i.currentField = NewField()
	i.currentField.Name = i.definition.FieldDefinitionNameString(ref)
	i.currentField.Description = i.definition.FieldDefinitionDescriptionString(ref)
	i.currentField.Type = i.TypeRef(i.definition.FieldDefinitionType(ref))

	if i.definition.FieldDefinitionHasDirectives(ref) {
		directiveRef, exists := i.definition.FieldDefinitionDirectiveByName(ref, []byte(DeprecatedDirectiveName))
		if exists {
			i.currentField.IsDeprecated = true
			i.currentField.DeprecationReason = i.deprecationReason(directiveRef)
		}
	}
}

func (i *introspectionVisitor) LeaveFieldDefinition(ref int) {
	if strings.HasPrefix(i.currentField.Name, "__") {
		return
	}
	i.currentType.Fields = append(i.currentType.Fields, i.currentField)
}

func (i *introspectionVisitor) EnterInputValueDefinition(ref int) {
	var defaultValue *string
	if i.definition.InputValueDefinitionHasDefaultValue(ref) {
		value := i.definition.InputValueDefinitionDefaultValue(ref)
		printedValue, err := i.definition.PrintValueBytes(value, nil)
		if err != nil {
			i.StopWithInternalErr(err)
			return
		}
		printedStr := unsafebytes.BytesToString(printedValue)
		defaultValue = &printedStr
	}

	inputValue := InputValue{
		Name:         i.definition.InputValueDefinitionNameString(ref),
		Description:  i.definition.InputValueDefinitionDescriptionString(ref),
		Type:         i.TypeRef(i.definition.InputValueDefinitionType(ref)),
		DefaultValue: defaultValue,
		TypeName:     "__InputValue",
	}

	if i.definition.InputValueDefinitionHasDirectives(ref) {
		directiveRef, exists := i.definition.InputValueDefinitionDirectiveByName(ref, []byte(DeprecatedDirectiveName))
		if exists {
			inputValue.IsDeprecated = true
			inputValue.DeprecationReason = i.deprecationReason(directiveRef)
		}
	}

	switch i.Ancestors[len(i.Ancestors)-1].Kind {
	case ast.NodeKindInputObjectTypeDefinition:
		i.currentType.InputFields = append(i.currentType.InputFields, inputValue)
	case ast.NodeKindFieldDefinition:
		i.currentField.Args = append(i.currentField.Args, inputValue)
	case ast.NodeKindDirectiveDefinition:
		i.currentDirective.Args = append(i.currentDirective.Args, inputValue)
	}
}

func (i *introspectionVisitor) EnterInterfaceTypeDefinition(ref int) {
	i.currentType = NewFullType()
	i.currentType.Kind = INTERFACE
	i.currentType.Name = i.definition.InterfaceTypeDefinitionNameString(ref)
	i.currentType.Description = i.definition.InterfaceTypeDefinitionDescriptionString(ref)

	interfaceNameBytes := i.definition.InterfaceTypeDefinitionNameBytes(ref)
	for objectTypeDefRef := range i.definition.ObjectTypeDefinitions {
		if i.definition.ObjectTypeDefinitionImplementsInterface(objectTypeDefRef, interfaceNameBytes) {
			objectName := i.definition.ObjectTypeDefinitionNameString(objectTypeDefRef)
			i.currentType.PossibleTypes = append(i.currentType.PossibleTypes, TypeRef{
				Kind:     OBJECT,
				Name:     &objectName,
				TypeName: "__Type",
			})
		}
	}

	for _, interfaceTypeExtension := range i.definition.InterfaceTypeExtensions {
		interfaceTypeExtensionName := i.definition.Input.ByteSliceString(interfaceTypeExtension.Name)
		for _, implementedInterfaceRef := range interfaceTypeExtension.ImplementsInterfaces.Refs {
			if i.currentType.Name == interfaceTypeExtensionName {
				implementedInterfaceName := i.definition.TypeNameString(implementedInterfaceRef)
				i.currentType.Interfaces = append(i.currentType.Interfaces, TypeRef{
					Kind:     INTERFACE,
					Name:     &implementedInterfaceName,
					TypeName: "__Type",
				})
			}
		}
	}

	for _, implementedInterfaceRef := range i.definition.InterfaceTypeDefinitions[ref].ImplementsInterfaces.Refs {
		implementedInterfaceName := i.definition.TypeNameString(implementedInterfaceRef)
		i.currentType.Interfaces = append(i.currentType.Interfaces, TypeRef{
			Kind:     INTERFACE,
			Name:     &implementedInterfaceName,
			TypeName: "__Type",
		})
	}
}

func (i *introspectionVisitor) LeaveInterfaceTypeDefinition(ref int) {
	if strings.HasPrefix(i.currentType.Name, "__") {
		return
	}
	i.data.Schema.AddType(i.currentType)
}

func (i *introspectionVisitor) EnterScalarTypeDefinition(ref int) {
	typeDefinition := NewFullType()
	typeDefinition.Kind = SCALAR
	typeDefinition.Name = i.definition.ScalarTypeDefinitionNameString(ref)
	typeDefinition.Description = i.definition.ScalarTypeDefinitionDescriptionString(ref)
	i.data.Schema.AddType(typeDefinition)

	if !i.definition.ScalarTypeDefinitionHasDirectives(ref) {
		return
	}

	directiveRef, exists := i.definition.ScalarTypeDefinitionDirectiveByName(ref, []byte(SpecifiedByDirectiveName))
	if !exists {
		return
	}

	argValue, exists := i.definition.DirectiveArgumentValueByName(directiveRef, []byte("url"))
	if !exists {
		return
	}

	url := i.definition.ValueContentString(argValue)
	typeDefinition.SpecifiedByURL = &url
}

func (i *introspectionVisitor) EnterUnionTypeDefinition(ref int) {
	i.currentType = NewFullType()
	i.currentType.Kind = UNION
	i.currentType.Name = i.definition.UnionTypeDefinitionNameString(ref)
	i.currentType.Description = i.definition.UnionTypeDefinitionDescriptionString(ref)
}

func (i *introspectionVisitor) LeaveUnionTypeDefinition(ref int) {
	if strings.HasPrefix(i.currentType.Name, "__") {
		return
	}
	i.data.Schema.AddType(i.currentType)
}

func (i *introspectionVisitor) EnterUnionMemberType(ref int) {
	name := i.definition.TypeNameString(ref)
	i.currentType.PossibleTypes = append(i.currentType.PossibleTypes, TypeRef{
		Kind:     OBJECT,
		Name:     &name,
		TypeName: "__Type",
	})
}

func (i *introspectionVisitor) EnterEnumTypeDefinition(ref int) {
	i.currentType = NewFullType()
	i.currentType.Kind = ENUM
	i.currentType.Name = i.definition.EnumTypeDefinitionNameString(ref)
	i.currentType.Description = i.definition.EnumTypeDefinitionDescriptionString(ref)
}

func (i *introspectionVisitor) LeaveEnumTypeDefinition(ref int) {
	if strings.HasPrefix(i.currentType.Name, "__") {
		return
	}
	i.data.Schema.AddType(i.currentType)
}

func (i *introspectionVisitor) LeaveEnumValueDefinition(ref int) {
	enumValue := EnumValue{
		Name:        i.definition.EnumValueDefinitionNameString(ref),
		Description: i.definition.EnumValueDefinitionDescriptionString(ref),
		TypeName:    "__EnumValue",
	}

	if i.definition.EnumValueDefinitionHasDirectives(ref) {
		directiveRef, exists := i.definition.EnumValueDefinitionDirectiveByName(ref, []byte(DeprecatedDirectiveName))
		if exists {
			enumValue.IsDeprecated = true
			enumValue.DeprecationReason = i.deprecationReason(directiveRef)
		}
	}

	i.currentType.EnumValues = append(i.currentType.EnumValues, enumValue)
}

func (i *introspectionVisitor) EnterInputObjectTypeDefinition(ref int) {
	i.currentType = NewFullType()
	i.currentType.Kind = INPUTOBJECT
	i.currentType.Name = i.definition.InputObjectTypeDefinitionNameString(ref)
	i.currentType.Description = i.definition.InputObjectTypeDefinitionDescriptionString(ref)
}

func (i *introspectionVisitor) LeaveInputObjectTypeDefinition(ref int) {
	i.data.Schema.AddType(i.currentType)
}

func (i *introspectionVisitor) EnterDirectiveDefinition(ref int) {
	i.currentDirective = NewDirective()
	i.currentDirective.Name = i.definition.DirectiveDefinitionNameString(ref)
	i.currentDirective.Description = i.definition.DirectiveDefinitionDescriptionString(ref)
	i.currentDirective.IsRepeatable = i.definition.DirectiveDefinitions[ref].Repeatable.IsRepeatable
}

func (i *introspectionVisitor) LeaveDirectiveDefinition(ref int) {
	i.data.Schema.Directives = append(i.data.Schema.Directives, i.currentDirective)
}

func (i *introspectionVisitor) EnterDirectiveLocation(location ast.DirectiveLocation) {
	i.currentDirective.Locations = append(i.currentDirective.Locations, location.LiteralString())
}

func (i *introspectionVisitor) EnterRootOperationTypeDefinition(ref int) {
	switch i.definition.RootOperationTypeDefinitions[ref].OperationType {
	case ast.OperationTypeQuery:
		i.queryTypeName = i.definition.Input.ByteSliceString(i.definition.RootOperationTypeDefinitions[ref].NamedType.Name)
	case ast.OperationTypeMutation:
		i.mutationTypeName = i.definition.Input.ByteSliceString(i.definition.RootOperationTypeDefinitions[ref].NamedType.Name)
	case ast.OperationTypeSubscription:
		i.subscriptionTypeName = i.definition.Input.ByteSliceString(i.definition.RootOperationTypeDefinitions[ref].NamedType.Name)
	default:
	}
}

func (i *introspectionVisitor) TypeRef(typeRef int) TypeRef {
	switch i.definition.Types[typeRef].TypeKind {
	case ast.TypeKindNamed:
		name := i.definition.TypeNameBytes(typeRef)
		node, exists := i.definition.Index.FirstNodeByNameBytes(name)
		if !exists {
			return TypeRef{TypeName: "__Type"}
		}
		var typeKind __TypeKind
		switch node.Kind {
		case ast.NodeKindScalarTypeDefinition:
			typeKind = SCALAR
		case ast.NodeKindObjectTypeDefinition:
			typeKind = OBJECT
		case ast.NodeKindEnumTypeDefinition:
			typeKind = ENUM
		case ast.NodeKindInterfaceTypeDefinition:
			typeKind = INTERFACE
		case ast.NodeKindUnionTypeDefinition:
			typeKind = UNION
		case ast.NodeKindInputObjectTypeDefinition:
			typeKind = INPUTOBJECT
		}
		nameStr := unsafebytes.BytesToString(name)
		return TypeRef{
			Kind:     typeKind,
			Name:     &nameStr,
			TypeName: "__Type",
		}
	case ast.TypeKindNonNull:
		ofType := i.TypeRef(i.definition.Types[typeRef].OfType)
		return TypeRef{
			Kind:     NONNULL,
			OfType:   &ofType,
			TypeName: "__Type",
		}
	case ast.TypeKindList:
		ofType := i.TypeRef(i.definition.Types[typeRef].OfType)
		return TypeRef{
			Kind:     LIST,
			OfType:   &ofType,
			TypeName: "__Type",
		}
	default:
		return TypeRef{TypeName: "__Type"}
	}
}

func (i *introspectionVisitor) deprecationReason(directiveRef int) (reason *string) {
	argValue, exists := i.definition.DirectiveArgumentValueByName(directiveRef, []byte(DeprecationReasonArgName))
	if exists {
		reasonContent := i.definition.ValueContentString(argValue)
		return &reasonContent
	}

	defaultValue := i.definition.DirectiveDefinitionArgumentDefaultValueString(DeprecatedDirectiveName, DeprecationReasonArgName)
	if defaultValue != "" {
		return &defaultValue
	}

	return
}
