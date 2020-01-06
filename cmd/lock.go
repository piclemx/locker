/*
Copyright © 2019 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"cirello.io/dynamolock"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"log"
	"time"

	"github.com/spf13/cobra"
)

var lockKey string
var lockOwner string
var data  map[string]string

// lockCmd represents the lock command
var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		region, err  := rootCmd.PersistentFlags().GetString("region")

		if err != nil {
			fmt.Println("Unable to set the region")
		}

		if lockOwner == "" {
			log.Fatalln("The owner should be set")
		}


		svc := dynamodb.New(session.Must(session.NewSession(&aws.Config{
			Region: aws.String(region),
		})))
		c, err := dynamolock.New(svc,
			"locks",
			dynamolock.WithLeaseDuration(3*time.Second),
			dynamolock.WithHeartbeatPeriod(1*time.Second),
			dynamolock.WithOwnerName(lockOwner),
		)
		if err != nil {
			log.Fatal(err)
		}
		defer c.Close()

		data, err := json.Marshal(data)
		if err != nil {
			fmt.Println("Wasn't able to serialize the data")
			return
		}
		lockedItem, err := c.AcquireLock(lockKey,
			dynamolock.WithData(data),
			dynamolock.ReplaceData(),
		)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("lock content:", string(lockedItem.Data()))
		if got := string(lockedItem.Data()); string(data) != got {
			log.Println("losing information inside lock storage, wanted:", string(data), " got:", got)
		}
	},
}

func init() {
	rootCmd.AddCommand(lockCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// lockCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	lockCmd.Flags().StringVarP(&lockKey, "key", "k","", "Key of the lock")
	lockCmd.Flags().StringToStringVar(&data, "data", map[string]string{}, "Data that will be stored with the lock")
	lockCmd.Flags().StringVarP(&lockOwner, "owner", "o", "", "Owner of the lock")
}
