package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/JaySon-Huang/tiflash-ctl/pkg/tidb"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
)

type FetchRegionsOpts struct {
	tidb            tidb.TiDBClientOpts
	tiflashHttpPort int
	dbName          string
	tableName       string
}

type ExecCmdOpts struct {
	tidb            tidb.TiDBClientOpts
	tiflashHttpPort int
	flashCmd        string
}

func newDispatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dispatch",
		Short: "Dispatch some actions for each TiFlash server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	/// Fetch Regions info for a table from all TiFlash instances
	newGetRegionCmd := func() *cobra.Command {
		var opt FetchRegionsOpts
		c := &cobra.Command{
			Use:   "fetch_region",
			Short: "Fetch Regions info for each TiFlash server",
			RunE: func(cmd *cobra.Command, args []string) error {
				return dumpTiFlashRegionInfo(opt)
			},
		}
		// Flags for "fetch region"
		addTiDBConnFlags(c, &opt.tidb)
		c.Flags().IntVar(&opt.tiflashHttpPort, "tiflash_http_port", 8123, "The port of TiFlash instance")

		c.Flags().StringVar(&opt.dbName, "database", "", "The database name of query table")
		c.Flags().StringVar(&opt.tableName, "table", "", "The table name of query table")
		return c
	}

	/// TODO: Apply delta merge for a table for all TiFlash instances

	newExecCmd := func() *cobra.Command {
		var opt ExecCmdOpts
		c := &cobra.Command{
			Use:   "exec",
			Short: "Exec command",
			RunE: func(cmd *cobra.Command, args []string) error {
				return execTiFlashCmd(opt)
			},
		}
		// Flags for "fetch region"
		addTiDBConnFlags(c, &opt.tidb)
		c.Flags().IntVar(&opt.tiflashHttpPort, "tiflash_http_port", 8123, "The port of TiFlash instance")

		c.Flags().StringVar(&opt.flashCmd, "cmd", "", "The command executed in all TiFlash")
		return c
	}

	cmd.AddCommand(newGetRegionCmd(), newExecCmd())

	return cmd
}

func getIPs(instances []string) []string {
	var IPs []string
	for _, s := range instances {
		sp := strings.Split(s, ":")
		IPs = append(IPs, sp[0])
	}
	return IPs
}

func curlTiFlash(ip string, httpPort int, query string) error {
	reqBodyReader := strings.NewReader(query)
	resp, err := http.Post(fmt.Sprintf("http://%s:%d/post", ip, httpPort), "text/html", reqBodyReader)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func dumpTiFlashRegionInfo(opts FetchRegionsOpts) error {
	if opts.dbName == "" || opts.tableName == "" {
		return fmt.Errorf("should set the database name and table name for running")
	}

	client, err := tidb.NewClientFromOpts(opts.tidb)
	if err != nil {
		return err
	}
	defer client.Close()

	instances := client.GetInstances("tiflash")
	ips := getIPs(instances)

	tableID := client.GetTableID(opts.dbName, opts.tableName)
	for _, ip := range ips {
		fmt.Printf("TiFlash ip: %s:%d table: `%s`.`%s` table_id: %d; Dumping Regions of table\n", ip, opts.tiflashHttpPort, opts.dbName, opts.tableName, tableID)
		err = curlTiFlash(ip, opts.tiflashHttpPort, fmt.Sprintf("DBGInvoke dump_all_region(%d)", tableID))
		fmt.Printf("err: %v", err)
	}
	return nil
}

func execTiFlashCmd(opts ExecCmdOpts) error {
	client, err := tidb.NewClientFromOpts(opts.tidb)
	if err != nil {
		return err
	}
	defer client.Close()

	instances := client.GetInstances("tiflash")
	ips := getIPs(instances)

	for _, ip := range ips {
		fmt.Printf("TiFlash ip: %s:%d\n", ip, opts.tiflashHttpPort)
		if err = curlTiFlash(ip, opts.tiflashHttpPort, opts.flashCmd); err != nil {
			fmt.Printf("err: %v\n", err)
		}
	}

	return nil
}
