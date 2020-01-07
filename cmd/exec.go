/*
Copyright © 2019 NAME HERE <alexandre.picardlemieux@gmail.com>

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
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var lockTimeout time.Duration
var heartbeatPeriod time.Duration
var dynamoLockLogger dynamolock.Logger
var waitForLock bool
var releaseOnError bool

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Lock execute the command and unlock",
	Args: func(cmd *cobra.Command, args []string) error {
		lockName := args[0]
		command := args[1:]
		if lockName == "" {
			return xerrors.New("missing lock name")
		}

		if region == "" {
			return  xerrors.New("unable to set the region")
		}

		if len(command) == 0 {
			return xerrors.New("missing command")
		}

		return nil
	},
	Long: `This command make it possible to lock before executing the command. After the command has successfully finish it will released the lock`,
	Run: func(cmd *cobra.Command, args []string) {
		lockName := args[0]
		ui.Info("Lock name"+ lockName)

		command := args[1:]

		ui.Info("Command: " + strings.Join(command, " "))

		client, err := dialDynamoDB(tableName)
		if err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}

		if err := createTable(client, tableName); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
		lock, err := grabLock(client, lockName, waitForLock)
		if err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		trap := make(chan os.Signal, 1)
		signal.Notify(trap, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-trap
			cancel()
		}()

		err = runCommand(ctx, lock, releaseOnError, command)

		if err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(execCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// lockCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	execCmd.Flags().DurationVar(&lockTimeout, "lock-timeout", time.Minute*10, "this will try to acquire a lock for at least 10 minutes before giving up and returning an error")
	execCmd.Flags().DurationVar(&heartbeatPeriod, "heartbeatPerdiod", time.Second * 5, "WithHeartbeatPeriod defines the frequency of the heartbeats. Set to zero to disable it. Heartbeats should have no more than half of the duration of the lease.")
	execCmd.Flags().BoolVarP(&waitForLock, "wait-for-lock", "w", true, "Wait for the lock, otherwise will exit if the lock is in used")
	execCmd.Flags().BoolVarP(&releaseOnError, "release-on-error", "r", true, "Release the lock if an error occurs when executing the command")
}

func createTable(client *dynamolock.Client, tableName string) error {
	_, err := client.CreateTable(tableName,
		dynamolock.WithProvisionedThroughput(&dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)
	if err != nil {
		var awsErr awserr.RequestFailure
		isTableAlreadyCreatedError := xerrors.As(err, &awsErr) && awsErr.StatusCode() == 400 && awsErr.Code() == "ResourceInUseException"
		if !isTableAlreadyCreatedError {
			return xerrors.Errorf("cannot create dynamolock client table: %w", err)
		}
	}
	return nil
}

func dialDynamoDB(tableName string) (*dynamolock.Client, error) {
	svc := dynamodb.New(session.Must(session.NewSession(&aws.Config{
		LogLevel: aws.LogLevel(aws.LogDebug),
		Region: aws.String(region),
	})))



	client, err := dynamolock.New(svc,
		tableName,
		dynamolock.WithLeaseDuration(lockTimeout),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithLogger(&DynamoLockLoggerClient{}),
	)
	if err != nil {
		return nil, xerrors.Errorf("cannot start dynamolock client: %w", err)
	}
	return client, nil
}

func grabLock(client *dynamolock.Client, lockName string, wait bool) (*dynamolock.Lock, error) {
	for {
		lock, err := client.AcquireLock(lockName, dynamolock.WithAdditionalTimeToWaitForLock(lockTimeout), dynamolock.WithDeleteLockOnRelease())
		if err != nil && wait {
			continue
		} else if err != nil {
			return nil, xerrors.Errorf("cannot lock %s: %w", lockName, err)
		}
		return lock, err
	}
}

func runCommand(ctx context.Context, lock *dynamolock.Lock, releaseOnError bool, cmd []string) error {
	command := cmd[0]
	var parameters []string
	if len(cmd) > 1 {
		parameters = cmd[1:]
	}
	wrappedCommand := exec.CommandContext(ctx, command, parameters...)
	wrappedCommand.Stdin = os.Stdin
	wrappedCommand.Stdout = os.Stdout
	wrappedCommand.Stderr = os.Stderr
	if err := wrappedCommand.Run(); err != nil {
		if releaseOnError {
			ui.Running("errored, releasing lock")
			if lockErr := lock.Close(); lockErr != nil {
				ui.Error("cannot release lock after failure:" + lockErr.Error())
			}
		}
		return xerrors.Errorf("error: %w", err)
	}
	if lockErr := lock.Close(); lockErr != nil {
		ui.Error("cannot release lock after completion:" +  lockErr.Error())
	}
	return nil
}
