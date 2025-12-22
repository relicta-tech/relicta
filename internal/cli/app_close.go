package cli

import "fmt"

func closeApp(app cliApp) {
	if app == nil {
		return
	}
	if err := app.Close(); err != nil {
		printWarning(fmt.Sprintf("Failed to close app: %v", err))
	}
}
