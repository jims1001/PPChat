package main

import (
	"encoding/json"
	"fmt"
	"math"
)

type Shape interface {
	Area() float64
}

type Rectangle struct {
	width, height float64
}

type Circle struct {
	radius float64
}

func (r Rectangle) Area() float64 {
	return r.width * r.height
}

func (c Circle) Area() float64 {
	return math.Pi * c.radius * c.radius
}

type Product struct {
	Name  string
	Price float64
}

type Persion struct {
	Name string  `json:"name"`
	Age  float64 `json:"age"`
}

func main() {
	print("hello world")

	r := Rectangle{width: 10, height: 5}
	c := Circle{radius: 5}
	println("\r\n")
	println(fmt.Sprintf("%2.0f", r.Area()))
	println(fmt.Sprintf("%3.5f", c.Area()))
	println(fmt.Sprintf("%3.0f", c.Area()))

	products := []Product{
		{"Apple", 5.2},
		{"Banana", 4.3},
		{"Orange", 3.0},
	}

	fmt.Printf("|%-15s|%10s|\n", "Product", "Price")
	fmt.Printf("--------------------------------------")
	println("\n")
	for _, p := range products {
		fmt.Printf("|%-15s|%10.2f|\n", p.Name, p.Price)
	}

	fmt.Printf("products: %+v\n", products)

	p := Persion{Name: "Alice", Age: 30}

	data, err := json.Marshal(p)
	if err != nil {
		fmt.Println("Error encoding JSON:", err)
		return
	}
	fmt.Println(string(data))

	jsonStr := `{"name":"Alice","age":30}`
	var p1 Persion
	err = json.Unmarshal([]byte(jsonStr), &p1)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return
	}
	fmt.Println(p.Name, p.Age)
}
