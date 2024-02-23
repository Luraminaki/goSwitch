package main

import (
	"fmt"
	"log"
	"os"

	"encoding/json"

	grid "goSwitch/modules"
)

type Config struct {
	Rows           int   `json:"Rows"`
	Cols           int   `json:"Cols"`
	ToggleSequence []int `json:"ToggleSequence"`
}

func main() {
	jsonFile, err := os.Open("./config.json")

	if err != nil {
		log.Fatal("Error when opening JSON file: ", err)
	}

	defer jsonFile.Close()

	if err != nil {
		log.Fatal("Error when reading JSON file: ", err)
	}

	var config Config
	err = json.NewDecoder(jsonFile).Decode(&config)

	if err != nil {
		log.Fatal("Error when parsing JSON file: ", err)
	}

	switchGame := grid.NewGrid(config.Cols, config.Rows)
	switchGame.PrettyPrintGrid()
	var x, y int
	for !switchGame.CheckWin() {

		fmt.Println("Input Col (x) Value")
		_, err = fmt.Scan(&x)
		if err != nil {
			log.Println("Error when reading input value: ", err)
			continue
		}

		fmt.Println("Input Row (y) Value")
		_, err = fmt.Scan(&y)
		if err != nil {
			log.Println("Error when reading input value: ", err)
			continue
		}

		fmt.Printf("Switching (%d,%d)\n\n", x, y)
		switchGame.SwitchCell(x, y, config.ToggleSequence)
		switchGame.PrettyPrintGrid()

		if switchGame.CheckWin() {
			fmt.Println("Did I Win: Yes")
		} else {
			fmt.Println("Did I Win: No")
		}
	}

	os.Exit(0)
}
