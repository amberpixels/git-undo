package app

func SetupInternalCall(app *App) {
	app.isInternalCall = true
}

func SetupAppDir(app *App, dir string) {
	app.dir = dir
}
