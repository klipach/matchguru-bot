package main

import (
	"log"

    "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
    _ "github.com/klipach/matchguru"
)

const port = "8082"

func main() {
    log.Println("Started")

    if err := funcframework.Start(port); err != nil {
        log.Fatalf("funcframework.Start: %v\n", err)
    }

    log.Println("Done")
}
