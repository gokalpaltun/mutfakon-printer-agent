package main

type printerInfo struct {
	Name      string  `json:"name"`
	Port      *string `json:"port"`
	Driver    *string `json:"driver"`
	IsDefault bool    `json:"isDefault"`
}
