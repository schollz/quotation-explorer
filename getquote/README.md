# Simple API for Quotes

Install:

```
go get github.com/schollz/quotation-explorer/getquote
```

Use:

```golang
package main

import (
	"fmt"

	"github.com/schollz/quotation-explorer/getquote"
)

func main() {
	fmt.Println(getquote.GetQuote())
}
```
