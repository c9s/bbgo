# bbgo Completion


## usage

```shell
(base) ➜  bbgo git:(main) ✗  bbgo completion -h

./build/bbgo/bbgo completion -h    
Generate the autocompletion script for bbgo for the specified shell.
See each sub-command's help for details on how to use the generated script.

Usage:
  bbgo completion [command]

Available Commands:
  bash        Generate the autocompletion script for bash
  fish        Generate the autocompletion script for fish
  powershell  Generate the autocompletion script for powershell
  zsh         Generate the autocompletion script for zsh

```

## shell configuration

```shell
(base) ➜  bbgo git:(main) ✗ ./build/bbgo/bbgo completion zsh -h


Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

        echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

        source <(bbgo completion zsh); compdef _bbgo bbgo

To load completions for every new session, execute once:

#### Linux:

        bbgo completion zsh > "${fpath[1]}/_bbgo"

#### macOS:

        bbgo completion zsh > $(brew --prefix)/share/zsh/site-functions/_bbgo

You will need to start a new shell for this setup to take effect.

```

## demo effect
Use the `tab` key to bring up the autocomplete prompt4


```shell
(base) ➜  bbgo git:(main) ✗ ./build/bbgo/bbgo account -
--binance-api-key              -- binance api key                                                                                                                                                                                                
--binance-api-secret           -- binance api secret                                                                                                                                                                                             
--config                       -- config file                                                                                                                                                                                                    
--cpu-profile                  -- cpu profile                                                                                                                                                                                                    
--debug                        -- debug mode                                                                                                                                                                                                     
--dotenv                       -- the dotenv file you want to load                                                                                                                                                                               
--ftx-api-key                  -- ftx api key                                                                                                                                                                                                    
--ftx-api-secret               -- ftx api secret                                                                                                                                                                                                 
--ftx-subaccount               -- subaccount name. Specify it if the crede
```
