package confed

import (
	"github.com/joho/godotenv"
)

type EnvEditor struct {
	keyValue map[string]string
}

func NewEnvEditor() *EnvEditor {
	return &EnvEditor{
		keyValue: make(map[string]string),
	}
}

func (e *EnvEditor) Read(path string) error {
	env, err := godotenv.Read(path)
	if err != nil {
		return err
	}
	e.keyValue = env
	return nil
}

func (e *EnvEditor) SetValue(key, value string) {
	e.keyValue[key] = value
}

func (e *EnvEditor) Write(path string) error {
	err := godotenv.Write(e.keyValue, path)
	if err != nil {
		return err
	}
	return nil
}
