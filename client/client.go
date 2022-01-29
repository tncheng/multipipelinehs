package main

import (
	multipipelinehs "github.com/tncheng/multipipelinehs"
	"github.com/tncheng/multipipelinehs/benchmark"
	"github.com/tncheng/multipipelinehs/db"
)

// Database implements multipipelinehs.DB interface for benchmarking
type Database struct {
	multipipelinehs.Client
}

func (d *Database) Init() error {
	return nil
}

func (d *Database) Stop() error {
	return nil
}

func (d *Database) Write(k int, v []byte) error {
	key := db.Key(k)
	err := d.Put(key, v)
	return err
}

func main() {
	multipipelinehs.Init()

	d := new(Database)
	d.Client = multipipelinehs.NewHTTPClient()
	b := benchmark.NewBenchmark(d)
	b.Run()
}
