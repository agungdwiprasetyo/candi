package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pkg.agungdp.dev/candi"
)

func projectGenerator(flagParam flagParameter, scope string, headerConfig configHeader,
	srvConfig serviceConfig, modConfigs []moduleConfig, baseConfig config) {

	srvConfig.configHeader = headerConfig
	srvConfig.config = baseConfig
	if scope == addModule || scope == addModuleMonorepoService {
		var baseDir string
		if flagParam.serviceName != "" {
			baseDir = flagParam.outputFlag + flagParam.serviceName + "/"
		}

		b, err := ioutil.ReadFile(baseDir + "candi.json")
		if err != nil {
			log.Fatal("ERROR: cannot find candi.json file")
		}
		json.Unmarshal(b, &srvConfig)
		for i := range srvConfig.Modules {
			srvConfig.Modules[i].Skip = true
		}
		modConfigs = append(modConfigs, srvConfig.Modules...)
	}

	sort.Slice(modConfigs, func(i, j int) bool {
		return modConfigs[i].ModuleName < modConfigs[j].ModuleName
	})
	srvConfig.Modules = modConfigs

	apiStructure := FileStructure{
		TargetDir: "api/", IsDir: true,
	}
	cmdStructure := FileStructure{
		TargetDir: "cmd/", IsDir: true,
		Childs: []FileStructure{
			{TargetDir: srvConfig.ServiceName + "/", IsDir: true, DataSource: srvConfig},
			{TargetDir: "migration/", IsDir: true, Childs: []FileStructure{
				{FromTemplate: true, DataSource: srvConfig, Source: templateCmdMigration, FileName: "migration.go", SkipFunc: func() bool {
					return !srvConfig.SQLDeps
				}},
			}},
		},
	}
	internalServiceStructure := FileStructure{
		TargetDir: "internal/", IsDir: true,
	}
	pkgServiceStructure := FileStructure{
		TargetDir: "pkg/", IsDir: true, DataSource: srvConfig,
	}
	apiProtoStructure := FileStructure{
		TargetDir: "proto/", IsDir: true, SkipFunc: func() bool { return !srvConfig.GRPCHandler },
	}
	apiGraphQLStructure := FileStructure{
		TargetDir: "graphql/", IsDir: true, SkipFunc: func() bool { return !srvConfig.GraphQLHandler },
	}

	moduleStructure := FileStructure{
		TargetDir: "modules/", IsDir: true, DataSource: srvConfig,
	}
	var sharedDomainFiles []FileStructure

	for _, mod := range srvConfig.Modules {
		mod.configHeader = srvConfig.configHeader
		mod.config = srvConfig.config
		if mod.Skip {
			continue
		}

		var repoModule = []FileStructure{
			{FromTemplate: true, DataSource: mod, Source: templateRepositoryAbstraction, FileName: "repository.go"},
		}
		repoModule = append(repoModule, parseRepositoryModule(mod)...)

		cleanArchModuleDir := []FileStructure{
			{
				TargetDir: "delivery/", IsDir: true,
				Childs: []FileStructure{
					{TargetDir: "graphqlhandler/", IsDir: true, SkipFunc: func() bool { return !srvConfig.GraphQLHandler },
						Childs: []FileStructure{
							{FromTemplate: true, DataSource: mod, Source: deliveryGraphqlRootTemplate, FileName: "root_resolver.go"},
							{FromTemplate: true, DataSource: mod, Source: deliveryGraphqlQueryTemplate, FileName: "query_resolver.go"},
							{FromTemplate: true, DataSource: mod, Source: deliveryGraphqlMutationTemplate, FileName: "mutation_resolver.go"},
							{FromTemplate: true, DataSource: mod, Source: deliveryGraphqlSubscriptionTemplate, FileName: "subscription_resolver.go"},
							{FromTemplate: true, DataSource: mod, Source: deliveryGraphqlFieldResolverTemplate, FileName: "field_serializer_resolver.go"},
						}},
					{TargetDir: "grpchandler/", IsDir: true, SkipFunc: func() bool { return !srvConfig.GRPCHandler },
						Childs: []FileStructure{
							{FromTemplate: true, DataSource: mod, Source: deliveryGRPCTemplate, FileName: "grpchandler.go"},
						}},
					{TargetDir: "resthandler/", IsDir: true, SkipFunc: func() bool { return !srvConfig.RestHandler },
						Childs: []FileStructure{
							{FromTemplate: true, DataSource: mod, Source: deliveryRestTemplate, FileName: "resthandler.go"},
						}},
					{TargetDir: "workerhandler/", IsDir: true,
						Childs: []FileStructure{
							{FromTemplate: true, DataSource: mod, Source: deliveryKafkaTemplate, FileName: "kafka_handler.go", SkipFunc: func() bool { return !srvConfig.KafkaHandler }},
							{FromTemplate: true, DataSource: mod, Source: deliveryRedisTemplate, FileName: "redis_handler.go", SkipFunc: func() bool { return !srvConfig.RedisSubsHandler }},
							{FromTemplate: true, DataSource: mod, Source: deliveryCronTemplate, FileName: "cron_handler.go", SkipFunc: func() bool { return !srvConfig.SchedulerHandler }},
							{FromTemplate: true, DataSource: mod, Source: deliveryTaskQueueTemplate, FileName: "taskqueue_handler.go", SkipFunc: func() bool { return !srvConfig.TaskQueueHandler }},
						}},
				},
			},
			{
				TargetDir: "domain/", IsDir: true,
				Childs: []FileStructure{
					{FromTemplate: true, FileName: "payload.go"},
				},
			},
			{
				TargetDir: "repository/", IsDir: true,
				Childs: repoModule,
			},
			{
				TargetDir: "usecase/", IsDir: true,
				Childs: []FileStructure{
					{FromTemplate: true, DataSource: mod, Source: templateUsecaseAbstraction, FileName: "usecase.go"},
					{FromTemplate: true, DataSource: mod, Source: templateUsecaseImpl, FileName: "usecase_impl.go"},
				},
			},
		}

		moduleStructure.Childs = append(moduleStructure.Childs, []FileStructure{
			{
				TargetDir: mod.ModuleName + "/", IsDir: true,
				Childs: append(cleanArchModuleDir,
					FileStructure{
						FromTemplate: true, DataSource: mod, Source: moduleMainTemplate, FileName: "module.go",
					},
				),
			},
		}...)

		apiProtoStructure.Childs = append(apiProtoStructure.Childs, FileStructure{
			TargetDir: mod.ModuleName + "/", IsDir: true,
			Childs: []FileStructure{
				{FromTemplate: true, DataSource: mod, Source: defaultGRPCProto, FileName: mod.ModuleName + ".proto"},
			},
		})
		apiGraphQLStructure.Childs = append(apiGraphQLStructure.Childs, FileStructure{
			FromTemplate: true, DataSource: mod, Source: defaultGraphqlSchema, FileName: mod.ModuleName + ".graphql",
		})

		// for shared domain
		sharedDomainFiles = append(sharedDomainFiles, FileStructure{
			FromTemplate: true, DataSource: mod, Source: templateDomain, FileName: mod.ModuleName + ".go",
		})
	}

	configsStructure := FileStructure{
		TargetDir: "configs/", IsDir: true,
		Childs: []FileStructure{
			{FromTemplate: true, DataSource: srvConfig, Source: configsTemplate, FileName: "configs.go"},
		},
	}

	internalServiceStructure.Childs = append(internalServiceStructure.Childs, FileStructure{
		FromTemplate: true, DataSource: srvConfig, Source: serviceMainTemplate, FileName: "service.go",
	})

	apiGraphQLStructure.Childs = append(apiGraphQLStructure.Childs, []FileStructure{
		{FromTemplate: true, DataSource: srvConfig, Source: defaultGraphqlRootSchema, FileName: "_schema.graphql"},
		{FromTemplate: true, DataSource: srvConfig, Source: templateGraphqlCommon, FileName: "_common.graphql"},
	}...)
	apiStructure.Childs = []FileStructure{
		apiGraphQLStructure,
		{
			TargetDir: "jsonschema/", IsDir: true,
			Childs: []FileStructure{
				{Source: jsonSchemaTemplate, FromTemplate: true, FileName: "schema.json"},
			},
		},
		apiProtoStructure,
	}

	configJSON, _ := json.Marshal(srvConfig)

	var baseDirectoryFile FileStructure
	baseDirectoryFile.TargetDir = flagParam.outputFlag + srvConfig.ServiceName + "/"
	baseDirectoryFile.DataSource = srvConfig
	baseDirectoryFile.IsDir = true
	switch {
	case scope == initService || scope == initMonorepoService:
		internalServiceStructure.Childs = append(internalServiceStructure.Childs, moduleStructure)
		pkgServiceStructure.Childs = append(pkgServiceStructure.Childs, []FileStructure{
			{TargetDir: "helper/", IsDir: true, Childs: []FileStructure{
				{FromTemplate: true, FileName: "helper.go"},
			}},
			{TargetDir: "shared/", IsDir: true, Childs: []FileStructure{
				{TargetDir: "domain/", IsDir: true, Childs: sharedDomainFiles},
				{TargetDir: "repository/", IsDir: true, Childs: parseSharedRepository(srvConfig)},
				{TargetDir: "usecase/", IsDir: true, Childs: []FileStructure{
					{FromTemplate: true, DataSource: srvConfig, Source: templateUsecaseUOW, FileName: "usecase.go"},
					{TargetDir: "common/", IsDir: true, Childs: []FileStructure{
						{FromTemplate: true, DataSource: srvConfig, Source: templateUsecaseCommon, FileName: "common.go"},
					}},
				}},
				{FromTemplate: true, DataSource: srvConfig, Source: templateSharedMiddlewareImpl, FileName: "middleware_impl_example.go"},
				{Source: additionalEnvTemplate, FromTemplate: true, DataSource: srvConfig, FileName: "environment.go"},
			}},
		}...)
		baseDirectoryFile.Childs = []FileStructure{
			apiStructure, cmdStructure, configsStructure, internalServiceStructure, pkgServiceStructure,
			{TargetDir: "deployments/", IsDir: true, Childs: []FileStructure{
				{TargetDir: "k8s/", IsDir: true, Childs: []FileStructure{
					{FileName: srvConfig.ServiceName + ".yaml"},
				}},
			}},
			{TargetDir: "docs/", IsDir: true, Childs: []FileStructure{
				{FileName: ".gitkeep"},
			}},
			{FromTemplate: true, DataSource: srvConfig, Source: gitignoreTemplate, FileName: ".gitignore"},
			{FromTemplate: true, DataSource: srvConfig, Source: makefileTemplate, FileName: "Makefile"},
			{FromTemplate: true, DataSource: srvConfig, Source: dockerfileTemplate, FileName: "Dockerfile", SkipFunc: func() bool {
				return isWorkdirMonorepo()
			}},
			{FromTemplate: true, DataSource: srvConfig, Source: cmdMainTemplate, FileName: "main.go"},
			{FromTemplate: true, DataSource: srvConfig, Source: envTemplate, FileName: ".env"},
			{FromTemplate: true, DataSource: srvConfig, Source: envTemplate, FileName: ".env.sample"},
			{Source: string(configJSON), FileName: "candi.json"},
			{FromTemplate: true, DataSource: srvConfig, Source: readmeTemplate, FileName: "README.md"},
		}
		if flagParam.withGoModFlag {
			baseDirectoryFile.Childs = append(baseDirectoryFile.Childs, FileStructure{
				FromTemplate: true, DataSource: srvConfig, Source: gomodTemplate, FileName: "go.mod",
			})
		}

	case scope == addModule || scope == addModuleMonorepoService:
		cmdStructure.Skip = true
		configsStructure.Skip = true
		moduleStructure.Skip = true
		pkgServiceStructure.Skip = true
		internalServiceStructure.Skip = true
		pkgServiceStructure.Childs = []FileStructure{
			{TargetDir: "shared/", IsDir: true, Skip: true, Childs: []FileStructure{
				{TargetDir: "domain/", IsDir: true, Skip: true, Childs: sharedDomainFiles},
				{TargetDir: "repository/", IsDir: true, Skip: true, Childs: parseSharedRepository(srvConfig)},
				{TargetDir: "usecase/", IsDir: true, Skip: true, Childs: []FileStructure{
					{FromTemplate: true, DataSource: srvConfig, Source: templateUsecaseUOW, FileName: "usecase.go"},
				}},
			}},
		}
		cmdStructure.Childs = []FileStructure{
			{TargetDir: "migration/", IsDir: true, Skip: true, Childs: []FileStructure{
				{FromTemplate: true, DataSource: srvConfig, Source: templateCmdMigration, FileName: "migration.go", SkipFunc: func() bool {
					return !srvConfig.SQLDeps
				}},
			}},
		}

		internalServiceStructure.Childs = append(internalServiceStructure.Childs, moduleStructure)
		apiStructure.Skip = true
		apiProtoStructure.Skip, apiGraphQLStructure.Skip = true, true
		apiStructure.Childs = []FileStructure{
			apiProtoStructure, apiGraphQLStructure,
		}

		baseDirectoryFile.Childs = append(baseDirectoryFile.Childs, []FileStructure{
			apiStructure, cmdStructure, internalServiceStructure, pkgServiceStructure,
		}...)
		baseDirectoryFile.Childs = append(baseDirectoryFile.Childs, FileStructure{
			Source: string(configJSON), FileName: "candi.json",
		})
		baseDirectoryFile.Skip = true
		baseDirectoryFile.TargetDir = ""
		if flagParam.serviceName != "" {
			baseDirectoryFile.TargetDir = flagParam.outputFlag + flagParam.serviceName + "/"
		}

	}

	execGenerator(baseDirectoryFile)
}

func monorepoGenerator(flagParam flagParameter) {
	var baseDirectoryFile FileStructure
	baseDirectoryFile.TargetDir = "monorepo/"
	baseDirectoryFile.IsDir = true
	baseDirectoryFile.Childs = []FileStructure{
		{TargetDir: "sdk/", IsDir: true, Childs: []FileStructure{{FileName: ".gitkeep"}}},
		{TargetDir: "services/", IsDir: true, Childs: []FileStructure{{FileName: ".gitkeep"}}},
		{FileName: "go.mod", Source: "module monorepo\n\ngo 1.15\n\nrequire pkg.agungdp.dev/candi " + candi.Version},
		{Source: gitignoreMonorepoTemplate, FileName: ".gitignore"},
		{Source: makefileMonorepoTemplate, FileName: "Makefile"},
		{Source: readmeMonorepoTemplate, FileName: "README.md"},
		{Source: dockerfileMonorepoTemplate, FileName: "Dockerfile"},
	}

	execGenerator(baseDirectoryFile)
}

func execGenerator(fl FileStructure) {
	if fl.Skip {
		goto execChild
	}

	if fl.SkipFunc != nil && fl.SkipFunc() {
		return
	}

	if _, err := os.Stat(fl.TargetDir); os.IsExist(err) {
		panic(err)
	}

	if fl.IsDir {
		fmt.Printf("creating %s...\n", fl.TargetDir)
		if err := os.Mkdir(fl.TargetDir, 0700); err != nil {
			fmt.Println("mkdir err:", err)
			panic(err)
		}
	}

	if fl.FileName != "" {
		var buff []byte
		if fl.FromTemplate {
			if fl.Source != "" {
				buff = loadTemplate(fl.Source, fl.DataSource)
			} else {
				lastDir := filepath.Dir(fl.TargetDir)
				buff = defaultDataSource(lastDir[strings.LastIndex(lastDir, "/")+1:])
			}
		} else {
			buff = []byte(fl.Source)
		}
		if err := ioutil.WriteFile(fl.TargetDir+fl.FileName, buff, 0644); err != nil {
			panic(err)
		}
	}

execChild:
	for _, child := range fl.Childs {
		child.TargetDir = fl.TargetDir + child.TargetDir
		execGenerator(child)
	}
}
