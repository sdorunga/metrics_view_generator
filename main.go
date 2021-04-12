package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"strings"

	"go.opencensus.io/stats"
)

var (
	SMSSentMetric = stats.Int64("sms/sent", "SMS Sent count", stats.UnitDimensionless) // Count
	PushSentMetric = stats.Float64("push/sent", "Push Sent count", stats.UnitMilliseconds) // Sum
)

var Emailmetric = stats.Int64("email/sent", "Email Sent count", stats.UnitBytes) // Distribution

func main() {
	file, err := ioutil.ReadFile("./main.go")
	if err != nil {
		panic(err)
	}
	pkgName, metrics := getMetricsFromBin(file)

	viewDefinitions := generateViewDefinitions(pkgName, metrics)
	ioutil.WriteFile(fmt.Sprintf("./metric_views.go"), []byte(viewDefinitions), 0644)
}

func generateViewDefinitions(pkgName string, merics []metricDef) string {
	var buf bytes.Buffer // Accumulated output.
	fmt.Fprint(&buf, "// AUTO-GENERATED FILE, DO NOT EDIT BY HAND\n")

	fmt.Fprintf(&buf, "package %s\n\n", pkgName)
	fmt.Fprint(&buf, "import (\n")
	fmt.Fprint(&buf, "  \"fmt\"\n\n")
	fmt.Fprint(&buf, "  \"go.opencensus.io/stats/view\"\n")
	fmt.Fprint(&buf, ")\n\n")

	fmt.Fprint(&buf, "var (\n")
	for _, metric := range merics {
		fmt.Fprintf(&buf, "  %sView = &view.View{\n", metric.VarName)
		fmt.Fprintf(&buf, "    Name: %s,\n", metric.Name)
		fmt.Fprintf(&buf, "    Measure: %s,\n", metric.VarName)
		fmt.Fprintf(&buf, "    Description: %s,\n", metric.Description)
		fmt.Fprintf(&buf, "    Aggregation: view.%s(),\n", metric.ViewAggregation)
		fmt.Fprint(&buf, "  }\n\n")
	}
	fmt.Fprint(&buf, ")\n\n")

	fmt.Fprint(&buf, "func registerMetrics() error {\n")
	fmt.Fprintf(&buf, "  if err := view.Register(%s); err != nil {\n", metricNames(merics))
	fmt.Fprint(&buf, "    return fmt.Errorf(\"failed to register metrics views: %w\", err)\n")
	fmt.Fprint(&buf, "  }\n")
	fmt.Fprint(&buf, "  return nil\n")
	fmt.Fprint(&buf, "}\n")

	return buf.String()
}

func metricNames(metrics []metricDef) string {
	str := []string{}
	for _, metric := range metrics {
		str = append(str, metric.VarName + "View")
	}

	return strings.Join(str, ", ")
}

type metricDef struct {
	VarName string
	Type string
	Name string
	Description string
	Unit string
	ViewAggregation string
}

func getMetricsFromBin(bytes []byte) (string, []metricDef) {
	fset := token.NewFileSet() // positions are relative to fset
	f, err := parser.ParseFile(fset, "src.go", bytes, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	metrics := []metricDef{}
	ast.Inspect(f, func(node ast.Node) bool {
		decl, ok := node.(*ast.GenDecl)
		if !ok || decl.Tok != token.VAR {
			//We only care about const declarations.
			return true
		}

		for _, spec := range decl.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				return true
			}
			if len(value.Values) == 0 {
				// No assignment to the var, just declaration
				return true
			}

			def, ok := value.Values[0].(*ast.CallExpr)
			if !ok {
				return true
			}

			metric, ok := getMetricFromDef(def, value.Comment, value.Names[0].Name)
			if !ok {
				return true
			}
			metrics = append(metrics, metric)
		}

		return true
	})
	return "main", metrics
}
func getMetricFromDef(def *ast.CallExpr, comment *ast.CommentGroup, name string) (metricDef, bool){
	if comment == nil {
		// BAD, should provide a metric type
		return metricDef{}, true
	}
	if len(comment.List) == 0 {
		// Comment list is empty
		return metricDef{}, true
	}
	aggregationText := comment.List[0]
	aggregationType := strings.TrimSpace(strings.TrimPrefix(aggregationText.Text, "//"))
	switch aggregationType {
	case "Count", "Sum", "Distribution":
		// Fallthrough for valid aggregations
	default:
		return metricDef{}, true
	}

	fun, ok := def.Fun.(*ast.SelectorExpr)
	if !ok {
		return metricDef{}, true
	}
	x, ok := fun.X.(*ast.Ident)
	if !ok {
		return metricDef{}, true
	}
	if x.Name != "stats" {
		// Skip, this is not a metric
		return metricDef{}, true
	}

	metricType := fun.Sel.Name
	switch metricType {
	case "Int64", "Float64":
		// Fallthrough for valid stats
	default:
		// Skip any declarations that don't match expected stat types
		return metricDef{}, true
	}
	statName, ok := def.Args[0].(*ast.BasicLit)
	if !ok {
		return metricDef{}, true
	}

	statDescription, ok := def.Args[1].(*ast.BasicLit)
	if !ok {
		return metricDef{}, true
	}

	arg3, ok := def.Args[2].(*ast.SelectorExpr)
	if !ok {
		return metricDef{}, true
	}
	x, ok = fun.X.(*ast.Ident)
	if !ok {
		return metricDef{}, true
	}
	if x.Name != "stats" {
		// Invalid dimension
		return metricDef{}, true
	}
	metricUnit := arg3.Sel.Name
	return metricDef{
		VarName: name,
		Type: metricType,
		Name: statName.Value,
		Description: statDescription.Value,
		Unit: metricUnit,
		ViewAggregation: aggregationType,
	}, true
}