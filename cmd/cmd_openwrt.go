package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hiddify/hiddify-core/v2/hcore"
	"github.com/hiddify/hiddify-core/v2/openwrt"
	"github.com/spf13/cobra"
)

var commandOpenWrt = &cobra.Command{
	Use:   "openwrt",
	Short: "Run HiddifyCli with embedded web server for OpenWrt",
	Run: func(cmd *cobra.Command, args []string) {
		webPort, _ := cmd.Flags().GetInt("web-port")
		webRoot, _ := cmd.Flags().GetString("web-root")
		disableAuth, _ := cmd.Flags().GetBool("disable-auth")

		err := hcore.Setup(&hcore.SetupRequest{
			BasePath:   ".",
			WorkingDir: ".",
			TempDir:    os.TempDir(),
			Mode:       hcore.SetupMode_GRPC_NORMAL_INSECURE,
		}, nil)
		if err != nil {
			log.Fatalf("Setup failed: %v", err)
		}

		grpcAddr := "127.0.0.1:17078"
		grpcServer, err := hcore.StartGrpcServerByMode(grpcAddr, hcore.SetupMode_GRPC_NORMAL_INSECURE)
		if err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
		log.Printf("gRPC server on %s", grpcAddr)

		var validate openwrt.SessionValidator
		if disableAuth {
			validate = func(token string) bool { return true }
			log.Println("WARNING: auth disabled")
		} else {
			validate = openwrt.ValidateRpcdSession
		}

		go func() {
			err := openwrt.StartWebServer(openwrt.ServerConfig{
				WebPort:    webPort,
				WebRoot:    webRoot,
				GRPCServer: grpcServer,
				Validate:   validate,
			})
			if err != nil {
				log.Fatalf("Web server failed: %v", err)
			}
		}()

		fmt.Printf("Hiddify OpenWrt ready on :%d (root: %s)\n", webPort, webRoot)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		log.Println("Shutting down...")
		hcore.CloseGrpcServer(hcore.SetupMode_GRPC_NORMAL_INSECURE)
	},
}

func init() {
	commandOpenWrt.Flags().Int("web-port", 8080, "Web server port")
	commandOpenWrt.Flags().String("web-root", "/www/hiddify", "Static files directory")
	commandOpenWrt.Flags().Bool("disable-auth", false, "Disable rpcd auth (for development)")
	mainCommand.AddCommand(commandOpenWrt)
}
