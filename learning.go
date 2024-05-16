package main // Creates an executable file

import "fmt" // fmt is a library to format i/o

/* When this program is executed the first function that runs is main.main() */
func main() {

	fruits := [...]string{"Apple", "Banana", "Strawberry", "Lemon", "Pineapple", "Orange", "Plum"}

	// define some slices

	classics := fruits[0:3]

	fmt.Println(classics)

}
