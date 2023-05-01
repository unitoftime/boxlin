package main

import (
	"math/rand"
)

type RngIntRange struct{
	Min, Max int
}
func (r RngIntRange) Roll() int {
	return rand.Intn(r.Max - r.Min) + r.Min
}

type RngItem[T any] struct{
	Weight int
	Item T
}
func NewRngItem[T any](weight int, item T) RngItem[T] {
	return RngItem[T]{
		Weight: weight,
		Item: item,
	}
}

type RngTable[T any] struct {
	Total int
	Items []RngItem[T]
}

func NewRngTable[T any](items ...RngItem[T]) *RngTable[T] {
	total := 0
	for i := range items {
		total += items[i].Weight
	}

	// TODO - Seeding?

	return &RngTable[T]{
		Total: total,
		Items: items, // TODO - maybe sort this. it might make the search a little faster?
	}
}

// Returns the item if successful, else returns nil
func (t *RngTable[T]) Roll() T {
	roll := rand.Intn(t.Total)

	// Essentially we just loop forward incrementing the `current` value. and once we pass it, we know that we are in that current section of the distribution.
	current := 0
	for i := range t.Items {
		current += t.Items[i].Weight
		if roll <= current {
			return t.Items[i].Item
		}
	}

	// TODO: is there a way to write this so it never fails?
	// Else just return the first item, something went wrong with the search
	return t.Items[0].Item
}
