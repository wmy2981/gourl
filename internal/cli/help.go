package cli

import (
	"fmt"
)

const helpText = `gourl — self-hosted URL shortener

USAGE
  gourl                      start the HTTP server (no subcommand)
  gourl <command> [options]  run an administrative command

COMMANDS
  help                       show this help
  version                    print the build version
  status                     show server state (config, database, redis)
  health                     check database and redis reachability
  config show                print the effective configuration (password hash hidden)
  setup-code                 print the bootstrap code of the running setup flow
                             (fails when the server is not in setup mode)
  log [lines]                print the last log lines from the mirrored file (default 100)
  db export [out-dir]        dump the SQLite database into links.json, tokens.json,
                             daily-clicks.json and backups.json (default out-dir: .)
  reset <target>             reset a configuration or data area (see below)
  webui on|off               enable or disable the admin console (/admin; /docs unaffected)
  restart                    stop the server so the container restarts it

SENSITIVE OPERATIONS
  Every reset target, webui on|off and restart prompt for confirmation on a
  terminal. Pass -y (or --yes) to skip the prompt for non-interactive use.
  Without a terminal and without -y the operation is refused.

RESET TARGETS
  password       clear the admin password and restart the service (setup mode)
  uablock        clear the blocked user-agent patterns
  ipblock        clear the banned IP rules
  config         delete the config file and restart the service (defaults)
  sessions       revoke every admin session (session epoch bump)
  api            revoke every API token (soft delete, like the API)
  db             delete the SQLite database and restart the service
  redis          wipe the Redis click buffer and restart the service
  --all          delete the data and config directories and restart the service
  (no target)    print this list

  reset password, config, db, redis, --all and restart stop the gourl
  process; the container restart policy starts it again — the confirmation
  prompt and the final message both say so. Click history lives in the
  database and is deleted with it.

ENVIRONMENT
  CONFIG_PATH  config file (default ./config/config.yaml)
  DB_PATH      SQLite database (default ./data/gourl.db)
  REDIS_ADDR   Redis address (default localhost:6379)
  LOG_DIR      log file directory (default ./data/log)

EXIT CODES
  0 success  1 error  2 usage
`

const resetHelpText = `usage: gourl reset <target> [-y]

RESET TARGETS
  password       clear the admin password and restart the service (setup mode)
  uablock        clear the blocked user-agent patterns
  ipblock        clear the banned IP rules
  config         delete the config file and restart the service (defaults)
  sessions       revoke every admin session (session epoch bump)
  api            revoke every API token (soft delete, like the API)
  db             delete the SQLite database and restart the service
  redis          wipe the Redis click buffer and restart the service
  --all          delete the data and config directories and restart the service
`

func printHelp() {
	fmt.Print(helpText)
}

func printResetHelp() {
	fmt.Print(resetHelpText)
}
