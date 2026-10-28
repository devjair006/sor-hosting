package executil
import("context";
"io")
type Executor interface{Run(context.Context,string,[]string,io.Reader,io.Writer)error}
