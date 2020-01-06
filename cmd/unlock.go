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
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"log"
	"time"

	"github.com/spf13/cobra"
)

var unlockKey string
var unlockOwner string

// unlockCmd represents the unlock command
var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock the given lock item",
	Long:  `Unlock the given lock item if the item is still their otherwise return`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("unlock called")

		region, err := rootCmd.PersistentFlags().GetString("region")

		if err != nil {
			fmt.Println("Unable to set the region")
		}

		if unlockOwner == "" {
			log.Fatalln("The owner should be set")
		}

		svc := dynamodb.New(session.Must(session.NewSession(&aws.Config{
			Region: aws.String(region),
		})))
		c, err := dynamolock.New(svc,
			"locks",
			dynamolock.WithLeaseDuration(3*time.Second),
			dynamolock.WithHeartbeatPeriod(1*time.Second),
			dynamolock.WithOwnerName(unlockOwner),
		)
		if err != nil {
			log.Fatal(err)
		}
		defer c.Close()

		lockedItem, lockErr := c.Get(unlockKey)

		if lockErr != nil {
			fmt.Println(lockErr.Error())
		}

		log.Println("cleaning lock")
		success, err := c.ReleaseLock(lockedItem)
		if !success {
			log.Fatal("lost lock before release")
		}
		if err != nil {
			log.Fatal("error releasing lock:", err)
		}
		log.Println("done")
	},
}

func init() {
	rootCmd.AddCommand(unlockCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// unlockCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// unlockCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	unlockCmd.Flags().StringVarP(&unlockKey, "key", "k", "", "Key of the lock")
	unlockCmd.Flags().StringVarP(&unlockOwner, "owner", "o", "", "Owner of the lock")
}
