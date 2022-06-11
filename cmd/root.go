/*
Copyright © 2022 Trilok Ramakrishna (trilok.ramakrishna@gmail.com)

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
	"crypto/sha1"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/cli/go-gh"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "gh aws-oidc",
	Short: "Connect Github to Azure for Workflow automation",
	Long:  `Connect Github to Azure for Workflow automation`,
	Run: func(cmd *cobra.Command, args []string) {
		orgrepo, _ := cmd.Flags().GetString("R")
		env, _ := cmd.Flags().GetString("e")
		orgFlag, _ := cmd.Flags().GetString("o")
		useDefaults, _ := cmd.Flags().GetString("useDefaults")

		var err error
		if orgrepo == "" {
			orgrepo, err = resolveRepository()
			if err != nil {
				panic(err)
			}
		}

		fmt.Printf("Setting up AWS Connection for %s\n\n", orgrepo)
		if useDefaults == "yes" {
			fmt.Println("Use Defaults option is still work in progress, we are progressing with the non default flow for now")
		}
		runSetup(orgrepo, env, orgFlag, useDefaults)

	},
}

func resolveRepository() (string, error) {
	args := []string{"repo", "view"}

	sout, eout, err := gh.Exec(args...)

	if err != nil {
		if strings.Contains(eout.String(), "not a git repository") {
			fmt.Println(err)
			return "", errors.New("Try running this command from inside a git repository or with the -R flag")
		}
		return "", err
	}
	viewOut := strings.Split(sout.String(), "\n")[0]
	repo := strings.TrimSpace(strings.Split(viewOut, ":")[1])

	return repo, nil
}

// // gh shells out to gh, returning STDOUT/STDERR and any error
// func gh(args ...string) (sout, eout bytes.Buffer, err error) {
// 	ghBin, err := safeexec.LookPath("gh")
// 	if err != nil {
// 		err = fmt.Errorf("could not find gh. Is it installed? error: %w", err)
// 		return
// 	}

// 	cmd := exec.Command(ghBin, args...)
// 	cmd.Stderr = &eout
// 	cmd.Stdout = &sout

// 	err = cmd.Run()
// 	if err != nil {
// 		err = fmt.Errorf("failed to run gh. error: %w, stderr: %s", err, eout.String())
// 		return
// 	}

// 	return
// }

func getFingerPrint(address string) [20]byte {

	conf := &tls.Config{
		InsecureSkipVerify: true, // as it may be self-signed
	}

	conn, err := tls.Dial("tcp", address, conf)
	if err != nil {
		log.Println("Error in Dial", err)
		// return ""
	}
	defer conn.Close()
	cert := conn.ConnectionState().PeerCertificates[0]
	fingerprint := sha1.Sum(cert.Raw)
	return fingerprint

}

func runSetup(orgrepo string, env string, orgFlag string, useDefaults string) {

	repoPrefix := "https://github.com"
	actionsURL := "vstoken.actions.githubusercontent.com"
	client, err := gh.RESTClient(nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	var repoName = orgrepo
	if repoName == "" {
		response := struct{ Login string }{}
		err = client.Get("user", &response)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("running as %s\n", response.Login)

		repoNameContent := promptContent{
			fmt.Sprintf("Enter the org/repo? "),
			fmt.Sprintf("Enter the org/repo? "),
		}

		repoName = promptGetInput(repoNameContent)
	}
	split := strings.Split(repoName, "/")
	roleNameContent := promptContent{
		fmt.Sprintf("Enter the Role Name (Default is gh-oidc-role-%v-%v) ", split[0], split[1]),
		fmt.Sprintf("Enter the Role Name (Default is gh-oidc-role-%v-%v) ", split[0], split[1]),
	}

	roleNameToCreate := promptGetInput(roleNameContent)
	if roleNameToCreate == "" {
		roleNameToCreate = fmt.Sprintf("gh-oidc-role-%v-%v) ", split[0], split[1])
	}

	thumbPrint := getFingerPrint(fmt.Sprintf(actionsURL + "443"))

	// Initialize a session in us-west-2 that the SDK will use to load
	// credentials from the shared credentials file ~/.aws/credentials.
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-west-2")},
	)

	fmt.Println("Creating AWS Resources")
	fmt.Println()

	// Create a IAM service client.
	svc := iam.New(sess)

	openIdProviders, err := svc.ListOpenIDConnectProviders(nil)
	found := false
	oidcIDPARNString := ""
	for _, value := range openIdProviders.OpenIDConnectProviderList {
		arnValue := value.Arn
		oidcIDPARNString = *arnValue
		// fmt.Println("ARN ", oidcIDPARNString)
		//Check if it matches the IDP being added
		if strings.Contains(oidcIDPARNString, actionsURL) {
			fmt.Println("IDP Already exists, Adding audience if its not added already")
			addClientIDRequest := iam.AddClientIDToOpenIDConnectProviderInput{
				ClientID:                 aws.String(repoPrefix + "/" + repoName),
				OpenIDConnectProviderArn: arnValue,
			}

			svc.AddClientIDToOpenIDConnectProvider(&addClientIDRequest)
			found = true
		}

	}

	if !found {
		oidcInput := &iam.CreateOpenIDConnectProviderInput{
			ClientIDList:   []*string{aws.String(repoPrefix + "/" + repoName)},
			ThumbprintList: []*string{aws.String(fmt.Sprintf("%x", thumbPrint))},
			Url:            aws.String("https://" + actionsURL),
		}

		oidcIDPOutput, err1 := svc.CreateOpenIDConnectProvider(oidcInput)
		if err1 != nil {
			fmt.Println("Error while creating OIDC Provider", err1)
			// return
		}

		newArn := oidcIDPOutput.OpenIDConnectProviderArn
		oidcIDPARNString = *newArn

		fmt.Println("Created OIDC Identity Provider with ARN: ", oidcIDPARNString)
	}

	b, err := ioutil.ReadFile("trust-policy-template.json") // just pass the file name
	if err != nil {
		fmt.Print(err)
	}

	rolePolicyDoc := string(b) // convert content to a 'string'

	fmt.Println(rolePolicyDoc) // print the content as a 'string'
	rolePolicyDoc = strings.ReplaceAll(rolePolicyDoc, "OIDCPROVIDER", oidcIDPARNString)
	rolePolicyDoc = strings.ReplaceAll(rolePolicyDoc, "AUDREPO", repoPrefix+"/"+repoName)
	rolePolicyDoc = strings.ReplaceAll(rolePolicyDoc, "AUDIENCE", repoName)

	// fmt.Println(rolePolicyDoc)

	//Create role
	roleInput := iam.GetRoleInput{
		RoleName: &roleNameToCreate,
	}

	roleArn := ""

	createRoleInput := iam.CreateRoleInput{
		RoleName:                 &roleNameToCreate,
		AssumeRolePolicyDocument: &rolePolicyDoc,
		Description:              aws.String("Role created by gh-aws-oidc cli"),
	}
	roleOutput, err := svc.GetRole(&roleInput)
	if roleOutput.Role != nil {
		fmt.Println("Not creating a new Role since it already exists")
		roleArnValue := roleOutput.Role.Arn
		roleArn = *roleArnValue
	} else {
		//Create Role
		// fmt.Println(createRoleInput)
		createRoleOutput, err := svc.CreateRole(&createRoleInput)
		if err != nil {
			fmt.Println("Error while Creating Role", err)
			return
		}
		roleArnValue := createRoleOutput.Role.Arn
		roleArn = *roleArnValue
		fmt.Println("Created Role With ARN", roleArn)
		attachRoleRequest := iam.AttachRolePolicyInput{
			PolicyArn: &roleArn,
			RoleName:  &roleNameToCreate,
		}

		//Attach role policy
		_, err1 := svc.AttachRolePolicy(&attachRoleRequest)
		if err1 != nil {
			fmt.Println("Error while attaching role policy", err1)
			return
		}
		fmt.Println("Attached Role With a Role Policy")

		// fmt.Println(attachRoleOutput)

	}

	fmt.Println("All AWS Resources Created Succesfully !")
	fmt.Println()
	fmt.Println("Now Updating your Repository secrets")

	createSecret("AWS_ROLE_ARN", roleArn, orgrepo, orgFlag, env)

	fmt.Println("Repository Updated Succesfully !")
	fmt.Println()
	fmt.Println("Succefully connected to AWS. You can now start executing your Actions Workflows !")
	time.Sleep(2 * time.Second)

}

func createSecret(name string, value string, orgrepo string, orgFlag string, env string) {
	args := []string{"secret", "set", name, "--body", value}
	if env != "" {
		args = append(args, "--env", env)
	}
	if orgFlag != "" {
		args = append(args, "--org", orgFlag)
	} else {
		args = append(args, "-R", orgrepo)
	}
	_, _, err := gh.Exec(args...)
	if err != nil {
		fmt.Println(err)
		return
	}
	// fmt.Println(stdOut.String())
}

type promptContent struct {
	errorMsg string
	label    string
}

func promptGetInput(pc promptContent) string {
	validate := func(input string) error {
		if len(input) <= 0 {
			return errors.New(pc.errorMsg)
		}
		return nil
	}

	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . }} ",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | red }} ",
		Success: "{{ . | bold }} ",
	}

	prompt := promptui.Prompt{
		Label:     pc.label,
		Templates: templates,
		Validate:  validate,
	}

	result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	// fmt.Printf("Input: %s\n", result)

	return result
}

func promptGetSelect(pc promptContent, items []string) string {
	index := -1
	var result string
	var err error

	for index < 0 {
		prompt := promptui.Select{
			Label: pc.label,
			Items: items,
			// AddLabel: "Other",
		}

		index, result, err = prompt.Run()

		if index == -1 {
			items = append(items, result)
		}
	}

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	// fmt.Printf("Input: %s\n", result)

	return result
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gh-aws-oidc.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".gh-aws-oidc" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".gh-aws-oidc")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
