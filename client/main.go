package main

import (
	"context"
	"github.com/dedinirtadinata/docxtool/docgenpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"log"
	"os"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	//untuk auth key
	md := metadata.Pairs("x-api-key", "secret-key-1")
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	c := docgenpb.NewDocServiceClient(conn)

	tpl, _ := os.ReadFile("template.docx")

	// Get placeholders
	ph, err := c.GetPlaceholders(ctx, &docgenpb.TemplateRequest{Template: tpl})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Placeholders:", ph.GetPlaceholders())

	// Generate PDF
	//ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	//defer cancel()
	resp, err := c.GeneratePDF(ctx, &docgenpb.GenerateRequest{
		Template: tpl,
		Data: map[string]string{
			"nama":    "Dedi",
			"alamat":  "Jl. Merdeka No. 1",
			"tanggal": "17 Agustus 2025",
		},
		FilenameHint: "surat",
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(resp.GetFilename(), resp.GetContent(), 0644); err != nil {
		log.Fatal(err)
	}
	log.Println("Saved:", resp.GetFilename())
}
