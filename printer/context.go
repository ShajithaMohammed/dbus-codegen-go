package printer

import (
	"errors"
	gotoken "go/token"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/amenzhinsky/dbus-codegen-go/token"
)

func newContext(ifaces []*token.Interface, opts ...PrintOption) (*context, error) {
	ctx := &context{
		PackageName: "dbusgen",
		Interfaces:  ifaces,
		gofmt:       true,
	}
	for _, opt := range opts {
		opt(ctx)
	}

	ctx.tpl = template.New("main").Funcs(template.FuncMap{
		"haveSignals":       ctx.tplHaveSignals,
		"ifaceNameConst":    ctx.tplIfaceNameConst,
		"ifaceType":         ctx.tplIfaceType,
		"unimplementedType": ctx.tplUnimplementedType,
		"serverType":        ctx.tplServerType,
		"methodType":        ctx.tplMethodType,
		"propType":          ctx.tplPropType,
		"propGetType":       ctx.tplPropGetType,
		"propSetType":       ctx.tplPropSetType,
		"propArgName":       ctx.tplPropArgName,
		"propNeedsGet":      ctx.tplPropNeedsGet,
		"propNeedsSet":      ctx.tplPropNeedsSet,
		"signalType":        ctx.tplSignalType,
		"signalBodyType":    ctx.tplSignalBodyType,
		"argName":           ctx.tplArgName,
		"joinMethodInArgs":  ctx.tplJoinMethodInArgs,
		"joinMethodOutArgs": ctx.tplJoinMethodOutArgs,
		"joinArgNames":      ctx.tplJoinArgNames,
		"joinStoreArgs":     ctx.tplJoinStoreArgs,
		"joinSignalValues":  ctx.tplJoinSignalValues,
		"joinSignalArgs":    ctx.tplJoinSignalArgs,
	})
	if !identRegexp.MatchString(ctx.PackageName) {
		return nil, errors.New("package name is not valid")
	}
	if len(ifaces) == 0 {
		return nil, errors.New("no interfaces given")
	}
	return ctx, nil
}

type context struct {
	PackageName string
	Imports     []string
	Interfaces  []*token.Interface
	ServerOnly  bool
	ClientOnly  bool

	tpl      *template.Template
	gofmt    bool
	camelize bool
	prefixes []string
}

// addImport adds a go import package.
func (ctx *context) addImport(pkg string) {
	for _, s := range ctx.Imports {
		if pkg == s {
			return
		}
	}
	ctx.Imports = append(ctx.Imports, pkg)
}

var ifaceRegexp = regexp.MustCompile(`[._][a-zA-Z0-9]`)

func (ctx *context) tplIfaceType(iface *token.Interface) string {
	name := iface.Name
	for _, prefix := range ctx.prefixes {
		if prefix[len(prefix)-1] == '.' {
			prefix = prefix[:len(prefix)-1]
		}
		if i := strings.Index(name, prefix); i != -1 {
			name = name[i+len(prefix)+1:]
			break
		}
	}
	name = strings.Title(name)
	if isKeyword(name) {
		return name
	}
	return ifaceRegexp.ReplaceAllStringFunc(name, func(s string) string {
		if ctx.camelize {
			return strings.ToUpper(s[1:])
		}
		return "_" + strings.ToUpper(s[1:])
	})
}

func (ctx *context) tplUnimplementedType(iface *token.Interface) string {
	return "Unimplemented" + ctx.tplIfaceType(iface)
}

func (ctx *context) tplServerType(iface *token.Interface) string {
	return ctx.tplIfaceType(iface) + "er"
}

func isKeyword(s string) bool {
	// TODO: validate it doesn't match imported package names
	return gotoken.Lookup(s).IsKeyword()
}

func (ctx *context) tplHaveSignals(ifaces []*token.Interface) bool {
	for _, iface := range ifaces {
		if len(iface.Signals) > 0 {
			return true
		}
	}
	return false
}

func (ctx *context) tplIfaceNameConst(iface *token.Interface) string {
	return "Interface" + ctx.tplIfaceType(iface)
}

func (ctx *context) tplMethodType(method *token.Method) string {
	return strings.Title(method.Name)
}

func (ctx *context) tplPropType(prop *token.Property) string {
	return strings.Title(prop.Name)
}

func (ctx *context) tplPropGetType(prop *token.Property) string {
	return "Get" + ctx.tplPropType(prop)
}

func (ctx *context) tplPropSetType(prop *token.Property) string {
	return "Set" + ctx.tplPropType(prop)
}

func (ctx *context) tplPropNeedsSet(iface *token.Interface, prop *token.Property) bool {
	if !prop.Write {
		return false
	}
	return ctx.propNeedsAccessor(iface, ctx.tplPropSetType(prop))
}

func (ctx *context) tplPropNeedsGet(iface *token.Interface, prop *token.Property) bool {
	if !prop.Read {
		return false
	}
	return ctx.propNeedsAccessor(iface, ctx.tplPropGetType(prop))
}

func (ctx *context) propNeedsAccessor(iface *token.Interface, name string) bool {
	for _, method := range iface.Methods {
		if method.Name == name {
			return false
		}
	}
	return true
}

func (ctx *context) tplSignalType(iface *token.Interface, signal *token.Signal) string {
	return ctx.tplIfaceType(iface) + "_" + strings.Title(signal.Name) + "Signal"
}

func (ctx *context) tplSignalBodyType(iface *token.Interface, signal *token.Signal) string {
	return ctx.tplSignalType(iface, signal) + "Body"
}

var varRegexp = regexp.MustCompile("_+[a-zA-Z0-9]")

func (ctx *context) tplArgName(arg *token.Arg, prefix string, i int, export bool) string {
	name := arg.Name
	if name == "" {
		name = prefix + strconv.Itoa(i)
	} else {
		name = strings.ToLower(name[:1]) +
			varRegexp.ReplaceAllStringFunc(name[1:], func(s string) string {
				return strings.Title(strings.TrimLeft(s, "_"))
			})
	}
	if export {
		name = strings.Title(name)
	}
	if isKeyword(name) {
		return prefix + strings.Title(name)
	}
	return name
}

func (ctx *context) tplPropArgName(prop *token.Property) string {
	return ctx.tplArgName(prop.Arg, "v", 0, false)
}

func (ctx *context) tplJoinStoreArgs(args []*token.Arg) string {
	var buf strings.Builder
	for i := range args {
		if i != 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('&')
		buf.WriteString(ctx.tplArgName(args[i], "out", i, false))
	}
	return buf.String()
}

func (ctx *context) tplJoinArgs(args []*token.Arg, separator byte, suffix string, export bool) string {
	var buf strings.Builder
	for i := range args {
		buf.WriteString(ctx.tplArgName(args[i], suffix, i, export))
		buf.WriteByte(' ')
		buf.WriteString(args[i].Type)
		buf.WriteByte(separator)
	}
	return buf.String()
}

func (ctx *context) tplJoinSignalValues(prefix string, sig *token.Signal) string {
	var buf strings.Builder
	for i := range sig.Args {
		buf.WriteString(prefix + ctx.tplArgName(sig.Args[i], "v", i, true))
		buf.WriteByte(',')
	}
	return buf.String()
}

func (ctx *context) tplJoinSignalArgs(sig *token.Signal) string {
	return ctx.tplJoinArgs(sig.Args, ';', "v", true)
}

func (ctx *context) tplJoinMethodInArgs(method *token.Method) string {
	return ctx.tplJoinArgs(method.In, ',', "in", false)
}

func (ctx *context) tplJoinMethodOutArgs(method *token.Method) string {
	return ctx.tplJoinArgs(method.Out, ',', "out", false)
}

func (ctx *context) tplJoinArgNames(args []*token.Arg) string {
	var buf strings.Builder
	for i := range args {
		if i != 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(ctx.tplArgName(args[i], "in", i, false))
	}
	return buf.String()
}
