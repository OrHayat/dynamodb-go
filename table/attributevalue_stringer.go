package table

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// indent is optional
func AttributevalueToString(av types.AttributeValue, indent *int) string {
	buf := bytes.Buffer{}
	attributevalueToString(av, &buf, indent, 0)
	return buf.String()
}

func AttributeValueMapToString(av map[string]types.AttributeValue, indent *int) string {
	buf := bytes.Buffer{}
	attributevalueMapToString(av, &buf, indent, 0)
	return buf.String()
}

func AttributeValueListToString(av []types.AttributeValue, indent *int) string {
	buf := bytes.Buffer{}
	attributevalueListToString(av, &buf, indent, 0)
	return buf.String()
}

func attributevalueListToString(
	av []types.AttributeValue,
	buf *bytes.Buffer,
	indent *int,
	depth int,
) {
	curPad, nextPad := getIndentation(depth, indent)
	buf.WriteString("List([")
	writeNewLineForIndent(indent, buf)

	for i, item := range av {
		buf.WriteString(nextPad)
		attributevalueToString(item, buf, indent, depth+1)
		if i < len(av)-1 {
			buf.WriteByte(',')
		}
		writeNewLineForIndent(indent, buf)
	}
	buf.WriteString(curPad + "])")

}

func attributevalueMapToString(
	av map[string]types.AttributeValue,
	buf *bytes.Buffer,
	indent *int,
	depth int,
) {
	curPad, nextPad := getIndentation(depth, indent)
	buf.WriteString("Map({")
	writeNewLineForIndent(indent, buf)
	var index = 0
	for k, val := range av {
		buf.WriteString(nextPad + strconv.Quote(k) + ":")
		attributevalueToString(val, buf, indent, depth+1)
		if index < len(av)-1 {
			buf.WriteByte(',')
		}
		writeNewLineForIndent(indent, buf)
		index++
	}
	buf.WriteString(curPad + "])")
}

func attributevalueToString(
	av types.AttributeValue,
	buf *bytes.Buffer,
	indent *int,
	depth int,
) {

	if av == nil {
		buf.WriteString("<nil>")
		return
	}

	switch convertedAv := av.(type) {
	case *types.AttributeValueMemberS:
		buf.WriteString("String(" + strconv.Quote(convertedAv.Value) + ")")
	case *types.AttributeValueMemberN:
		buf.WriteString("Number(" + convertedAv.Value + ")")
	case *types.AttributeValueMemberBOOL:
		buf.WriteString("Bool(" + strconv.FormatBool(convertedAv.Value) + ")")
	case *types.AttributeValueMemberB:
		buf.WriteString("Bytes(")
		buf.Write(convertedAv.Value)
		buf.WriteByte(')')
	case *types.AttributeValueMemberNULL:
		buf.WriteString("Null(" + strconv.FormatBool(convertedAv.Value) + ")")
	case *types.AttributeValueMemberSS:
		buf.WriteString("StringSet([")
		for i, val := range convertedAv.Value {
			buf.WriteString(strconv.Quote(val))
			if i < len(convertedAv.Value)-1 {
				buf.WriteByte(',')
			}
		}
		buf.WriteString("])")
	case *types.AttributeValueMemberNS:
		buf.WriteString("NumberSet([")
		for i, val := range convertedAv.Value {
			buf.WriteString(val)
			if i < len(convertedAv.Value)-1 {
				buf.WriteByte(',')
			}
		}
		buf.WriteString("])")
	case *types.AttributeValueMemberBS:
		buf.WriteString("BinarySet([")
		for i, val := range convertedAv.Value {
			buf.Write(val)
			if i < len(convertedAv.Value)-1 {
				buf.WriteByte(',')
			}
		}
		buf.WriteString("])")

	case *types.AttributeValueMemberL:
		attributevalueListToString(convertedAv.Value, buf, indent, depth)
	case *types.AttributeValueMemberM:
		attributevalueMapToString(convertedAv.Value, buf, indent, depth)
	default:
		buf.WriteString("<UnknownAttributeValue>:" + fmt.Sprintf("%#v", convertedAv))
	}
}

func getIndentation(depth int, indent *int) (cur string, next string) {
	if indent == nil {
		return "", ""
	}
	cur = strings.Repeat(" ", *indent*depth)
	next = cur + strings.Repeat(" ", *indent)
	return cur, next
}

func writeNewLineForIndent(indent *int, buf *bytes.Buffer) {
	if indent == nil {
		return
	}
	buf.WriteByte('\n')
}
