package env

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"pkg.agungdp.dev/candi/candihelper"
)

// Env model
type Env struct {
	RootApp, ServiceName string
	BuildNumber          string
	// Env on application
	Environment string

	useSQL, useMongo, useRedis, useRSAKey bool
	NoAuth                                bool

	// UseREST env
	UseREST bool
	// UseGraphQL env
	UseGraphQL bool
	// UseGRPC env
	UseGRPC bool
	// UseKafkaConsumer env
	UseKafkaConsumer bool
	// UseCronScheduler env
	UseCronScheduler bool
	// UseRedisSubscriber env
	UseRedisSubscriber bool
	// UseTaskQueueWorker env
	UseTaskQueueWorker bool

	// GraphQLSchemaDir env
	GraphQLSchemaDir string
	// JSONSchemaDir env
	JSONSchemaDir string

	IsProduction, DebugMode bool

	// HTTPPort config
	HTTPPort uint16
	// GRPCPort Config
	GRPCPort uint16
	// TaskQueueDashboardPort Config
	TaskQueueDashboardPort uint16
	// TaskQueueDashboardMaxClientSubscribers Config
	TaskQueueDashboardMaxClientSubscribers int

	// UseConsul for distributed lock if run in multiple instance
	UseConsul bool
	// ConsulAgentHost consul agent host
	ConsulAgentHost string
	// ConsulMaxJobRebalance env, if worker execute total job in env config, rebalance worker to another active intance
	ConsulMaxJobRebalance int

	// BasicAuthUsername config
	BasicAuthUsername string
	// BasicAuthPassword config
	BasicAuthPassword string

	// JaegerTracingHost env
	JaegerTracingHost string

	// Broker environment
	Kafka struct {
		Brokers       []string
		ClientVersion string
		ClientID      string
		ConsumerGroup string
	}

	// MaxGoroutines env for goroutine semaphore
	MaxGoroutines int

	// Database environment
	DbMongoWriteHost, DbMongoReadHost, DbMongoDatabaseName string
	DbSQLWriteDSN, DbSQLReadDSN                            string
	DbRedisReadDSN, DbRedisWriteDSN                        string
}

var env Env

// BaseEnv get global basic environment
func BaseEnv() Env {
	return env
}

// SetEnv set env for mocking data env
func SetEnv(newEnv Env) {
	env = newEnv
}

// Load environment
func Load(serviceName string) {
	var ok bool
	env.ServiceName = serviceName

	// load main .env and additional .env in app
	err := godotenv.Load(os.Getenv(candihelper.WORKDIR) + ".env")
	if err != nil {
		panic(fmt.Errorf("Load env: %v", err))
	}

	// ------------------------------------
	parseAppConfig()
	env.BuildNumber = os.Getenv("BUILD_NUMBER")

	// ------------------------------------
	if env.UseREST || env.UseGraphQL {
		if httpPort, ok := os.LookupEnv("HTTP_PORT"); !ok {
			panic("missing HTTP_PORT environment")
		} else {
			port, err := strconv.Atoi(httpPort)
			if err != nil {
				panic("HTTP_PORT environment must in integer format")
			}
			env.HTTPPort = uint16(port)
		}
	} else if env.UseGRPC {
		if _, ok = os.LookupEnv("GRPC_PORT"); !ok {
			panic("missing GRPC_PORT environment")
		}
	}

	port, _ := strconv.Atoi(os.Getenv("GRPC_PORT"))
	env.GRPCPort = uint16(port)

	if env.UseTaskQueueWorker {
		taskQueueDashboardPort, ok := os.LookupEnv("TASK_QUEUE_DASHBOARD_PORT")
		if !ok {
			taskQueueDashboardPort = "8080"
		}
		port, err := strconv.Atoi(taskQueueDashboardPort)
		if err != nil {
			panic("TASK_QUEUE_DASHBOARD_PORT environment must in integer format")
		}
		env.TaskQueueDashboardPort = uint16(port)
		env.TaskQueueDashboardMaxClientSubscribers, _ = strconv.Atoi(os.Getenv("TASK_QUEUE_DASHBOARD_MAX_CLIENT"))
		if env.TaskQueueDashboardPort <= 0 || env.TaskQueueDashboardMaxClientSubscribers > 10 {
			env.TaskQueueDashboardMaxClientSubscribers = 10 // default
		}
	}

	env.UseConsul, _ = strconv.ParseBool(os.Getenv("USE_CONSUL"))
	if env.UseConsul {
		env.ConsulAgentHost, ok = os.LookupEnv("CONSUL_AGENT_HOST")
		if !ok {
			panic("consul is active, missing CONSUL_AGENT_HOST environment")
		}
		env.ConsulMaxJobRebalance = 10
		if count, err := strconv.Atoi(os.Getenv("CONSUL_MAX_JOB_REBALANCE")); err == nil {
			env.ConsulMaxJobRebalance = count
		}
	}

	// ------------------------------------
	env.Environment = os.Getenv("ENVIRONMENT")
	env.IsProduction = strings.ToLower(env.Environment) == "production"
	env.DebugMode, err = strconv.ParseBool(os.Getenv("DEBUG_MODE"))
	if err != nil {
		env.DebugMode = true
	}
	env.NoAuth, _ = strconv.ParseBool(os.Getenv("NO_AUTH"))

	env.BasicAuthUsername, ok = os.LookupEnv("BASIC_AUTH_USERNAME")
	if !ok {
		panic("missing BASIC_AUTH_USERNAME environment")
	}
	env.BasicAuthPassword, ok = os.LookupEnv("BASIC_AUTH_PASS")
	if !ok {
		panic("missing BASIC_AUTH_PASS environment")
	}

	env.JaegerTracingHost, ok = os.LookupEnv("JAEGER_TRACING_HOST")
	if !ok {
		panic("missing JAEGER_TRACING_HOST environment")
	}

	// kafka environment
	parseBrokerEnv()

	env.GraphQLSchemaDir, ok = os.LookupEnv("GRAPHQL_SCHEMA_DIR")
	if env.UseGraphQL && !ok {
		panic("GRAPHQL is active, missing GRAPHQL_SCHEMA_DIR environment")
	}
	env.GraphQLSchemaDir = os.Getenv(candihelper.WORKDIR) + env.GraphQLSchemaDir

	env.JSONSchemaDir, ok = os.LookupEnv("JSON_SCHEMA_DIR")
	if !ok {
		panic("missing JSON_SCHEMA_DIR environment")
	}
	env.JSONSchemaDir = os.Getenv(candihelper.WORKDIR) + env.JSONSchemaDir

	maxGoroutines, err := strconv.Atoi(os.Getenv("MAX_GOROUTINES"))
	if err != nil || maxGoroutines <= 0 {
		maxGoroutines = 4096
	}
	env.MaxGoroutines = maxGoroutines

	// Parse database environment
	parseDatabaseEnv()
}

func parseAppConfig() {

	useREST, ok := os.LookupEnv("USE_REST")
	if !ok {
		flag.BoolVar(&env.UseREST, "USE_REST", false, "USE REST")
	} else {
		env.UseREST, _ = strconv.ParseBool(useREST)
	}

	useGraphQL, ok := os.LookupEnv("USE_GRAPHQL")
	if !ok {
		flag.BoolVar(&env.UseGraphQL, "USE_GRAPHQL", false, "USE GRAPHQL")
	} else {
		env.UseGraphQL, _ = strconv.ParseBool(useGraphQL)
	}

	useGRPC, ok := os.LookupEnv("USE_GRPC")
	if !ok {
		flag.BoolVar(&env.UseGRPC, "USE_GRPC", false, "USE GRPC")
	} else {
		env.UseGRPC, _ = strconv.ParseBool(useGRPC)
	}

	useKafkaConsumer, ok := os.LookupEnv("USE_KAFKA_CONSUMER")
	if !ok {
		flag.BoolVar(&env.UseKafkaConsumer, "USE_KAFKA_CONSUMER", false, "USE KAFKA CONSUMER")
	} else {
		env.UseKafkaConsumer, _ = strconv.ParseBool(useKafkaConsumer)
	}

	useCronScheduler, ok := os.LookupEnv("USE_CRON_SCHEDULER")
	if !ok {
		flag.BoolVar(&env.UseCronScheduler, "USE_CRON_SCHEDULER", false, "USE CRON SCHEDULER")
	} else {
		env.UseCronScheduler, _ = strconv.ParseBool(useCronScheduler)
	}

	useRedisSubs, ok := os.LookupEnv("USE_REDIS_SUBSCRIBER")
	if !ok {
		flag.BoolVar(&env.UseRedisSubscriber, "USE_REDIS_SUBSCRIBER", false, "USE REDIS SUBSCRIBER")
	} else {
		env.UseRedisSubscriber, _ = strconv.ParseBool(useRedisSubs)
	}

	useTaskQueue, ok := os.LookupEnv("USE_TASK_QUEUE_WORKER")
	if !ok {
		flag.BoolVar(&env.UseTaskQueueWorker, "USE_TASK_QUEUE_WORKER", false, "USE TASK QUEUE WORKER")
	} else {
		env.UseTaskQueueWorker, _ = strconv.ParseBool(useTaskQueue)
	}

	flag.Usage = func() {
		fmt.Println("	-USE_REST :=> Activate REST Server")
		fmt.Println("	-USE_GRPC :=> Activate GRPC Server")
		fmt.Println("	-USE_GRAPHQL :=> Activate GraphQL Server")
		fmt.Println("	-USE_KAFKA_CONSUMER :=> Activate Kafka Consumer Worker")
		fmt.Println("	-USE_CRON_SCHEDULER :=> Activate Cron Scheduler Worker")
		fmt.Println("	-USE_REDIS_SUBSCRIBER :=> Activate Redis Subscriber Worker")
		fmt.Println("	-USE_TASK_QUEUE_WORKER :=> Activate Task Queue Worker")
	}
	flag.Parse()
}

func parseBrokerEnv() {
	kafkaBrokerEnv := os.Getenv("KAFKA_BROKERS")
	env.Kafka.Brokers = strings.Split(kafkaBrokerEnv, ",") // optional
	env.Kafka.ClientID = os.Getenv("KAFKA_CLIENT_ID")      // optional
	env.Kafka.ClientVersion = os.Getenv("KAFKA_CLIENT_VERSION")
	if env.UseKafkaConsumer {
		if kafkaBrokerEnv == "" {
			panic("kafka consumer is active, missing KAFKA_BROKERS environment")
		}

		var ok bool
		env.Kafka.ConsumerGroup, ok = os.LookupEnv("KAFKA_CONSUMER_GROUP")
		if !ok {
			panic("kafka consumer is active, missing KAFKA_CONSUMER_GROUP environment")
		}
	}
}

func parseDatabaseEnv() {
	env.DbMongoWriteHost = os.Getenv("MONGODB_HOST_WRITE")
	env.DbMongoReadHost = os.Getenv("MONGODB_HOST_READ")
	env.DbMongoDatabaseName = os.Getenv("MONGODB_DATABASE_NAME")

	env.DbSQLReadDSN = os.Getenv("SQL_DB_READ_DSN")
	env.DbSQLWriteDSN = os.Getenv("SQL_DB_WRITE_DSN")

	env.DbRedisReadDSN = os.Getenv("REDIS_READ_DSN")
	env.DbRedisWriteDSN = os.Getenv("REDIS_WRITE_DSN")
}
