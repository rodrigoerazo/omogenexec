#include <iostream>
#include <string>
#include <vector>

#include "absl/strings/str_split.h"
#include "api/omogenexec.grpc.pb.h"
#include "gflags/gflags.h"
#include "glog/logging.h"
#include "glog/raw_logging.h"
#include "grpcpp/grpcpp.h"
#include "util/files.h"

using namespace omogenexec;

template <typename T>
static bool validateNonNegative(const char* flagname, T value) {
  if (value < 0) {
    LOG(ERROR) << "Invalid value for -" << flagname << ": " << value;
    return false;
  }
  return true;
}

static bool validateNonEmptyString(const char* flagname,
                                   const std::string& value) {
  if (value.empty()) {
    LOG(ERROR) << "Invalid value for -" << flagname << ": " << value;
    return false;
  }
  return true;
}

static bool validateDirectoryRules(const char* flagname,
                                   const std::string& value) {
  std::vector<std::string> rules = absl::StrSplit(value, ';');
  for (const std::string& rule : rules) {
    if (rule.empty()) {
      continue;
    }
    // Rules are of the format /path/to/include[:writable]
    std::vector<std::string> fields = absl::StrSplit(rule, ':');
    if (fields.size() < 1 || 2 < fields.size()) {
      LOG(ERROR) << "Directory rule " << rule << " invalid";
      return false;
    }
    if (fields[0].empty() || fields[0][0] != '/') {
      LOG(ERROR) << "Path " << fields[0] << " is not absolute";
      return false;
    }
    if (!omogenexec::DirectoryExists(fields[0])) {
      LOG(ERROR) << "Directory outside sandbox " << fields[0]
                 << " does not exist";
      return false;
    }
    if (fields.size() == 2 && fields[1] != "writable") {
      LOG(ERROR) << "Invalid directory options " << fields[1];
      return false;
    }
  }
  return true;
}

DEFINE_string(daemon_addr, "127.0.0.1:61810",
              "The address and port that the daemon listens to");
DEFINE_double(cputime, 10, "The CPU time limit of the process in seconds");
DEFINE_validator(cputime, &validateNonNegative<double>);
DEFINE_double(memory, 200, "The memory limit in megabytes (10^6 bytes)");
DEFINE_validator(memory, &validateNonNegative<double>);
DEFINE_int32(processes, 1, "The number of processes allowed");
DEFINE_validator(processes, &validateNonNegative<int>);
DEFINE_int32(repetitions, 1, "The number of times to run the process");
DEFINE_validator(repetitions, &validateNonNegative<int>);

DEFINE_string(in, "/dev/null", "An input file that is used as stdin");
DEFINE_validator(in, &validateNonEmptyString);
DEFINE_string(out, "/dev/null", "An output file that is used as stdout");
DEFINE_validator(out, &validateNonEmptyString);
DEFINE_string(err, "/dev/null", "An error file that is used as stderr");
DEFINE_validator(err, &validateNonEmptyString);

DEFINE_string(
    dirs, "",
    "Directory rules for mounting things inside the container. Format is "
    "path[:writable] separated by semicolon, e.g. /tmp:writable;/home");
DEFINE_validator(dirs, &validateDirectoryRules);

static void parseDirectoryRule(api::DirectoryMount* rule,
                               const std::string& ruleStr) {
  std::vector<std::string> fields = absl::StrSplit(ruleStr, ':');
  CHECK(1 <= fields.size() && fields.size() <= 2) << "Invalid directory rule";
  rule->set_path_outside_container(fields[0]);
  rule->set_path_inside_container(fields[0]);
  if (fields.size() == 2) {
    CHECK(fields[1] == "writable") << "Invalid directory rule";
    rule->set_writable(true);
  }
}

static void parseDirectoryRules(api::ContainerSpec* request,
                                const std::string& rulesStr) {
  std::vector<std::string> rules = absl::StrSplit(rulesStr, ';');
  for (const std::string& ruleStr : rules) {
    if (ruleStr.empty()) {
      return;
    }
    parseDirectoryRule(request->add_mounts(), ruleStr);
  }
}

static void setLimit(api::ResourceAmount* amount, api::ResourceType type,
                     long long limit) {
  amount->set_type(type);
  amount->set_amount(limit);
}

static void setLimits(api::ResourceAmounts* limits) {
  setLimit(limits->add_amounts(), api::ResourceType::CPU_TIME,
           FLAGS_cputime * 1000);
  setLimit(limits->add_amounts(), api::ResourceType::WALL_TIME,
           (FLAGS_cputime + 1) * 1000);
  setLimit(limits->add_amounts(), api::ResourceType::MEMORY,
           FLAGS_memory * 1000);
  setLimit(limits->add_amounts(), api::ResourceType::PROCESSES,
           FLAGS_processes);
}

static void addStream(api::Streams::Mapping* file, const std::string& path,
                      bool write) {
  file->mutable_file()->set_path_inside_container(path);
  file->set_write(write);
}

static void setStreams(api::Streams* streams) {
  addStream(streams->add_mappings(), FLAGS_in, false);
  addStream(streams->add_mappings(), FLAGS_out, true);
  addStream(streams->add_mappings(), FLAGS_err, true);
}

static void setCommand(api::Command* command, const std::string& path) {
  command->set_command(path);
}

static void addFlag(api::Command* command, const std::string& flag) {
  command->add_flags(flag);
}

int main(int argc, char** argv) {
  gflags::ParseCommandLineFlags(&argc, &argv, true);
  google::InitGoogleLogging(argv[0]);

  api::ExecuteRequest request;
  api::Execution* exec = request.mutable_execution();
  setLimits(exec->mutable_resource_limits());
  setStreams(exec->mutable_environment()->mutable_stream_redirections());
  parseDirectoryRules(request.mutable_container_spec(), FLAGS_dirs);

  if (argc < 2) {
    LOG(FATAL) << "No command provided";
  }
  setCommand(exec->mutable_command(), std::string(argv[1]));
  for (int i = 2; i < argc; ++i) {
    addFlag(exec->mutable_command(), std::string(argv[i]));
  }

  std::unique_ptr<api::ExecuteService::Stub> stub =
      api::ExecuteService::NewStub(grpc::CreateChannel(
          FLAGS_daemon_addr, grpc::InsecureChannelCredentials()));
  grpc::ClientContext context;
  std::shared_ptr<
      grpc::ClientReaderWriter<api::ExecuteRequest, api::ExecuteResponse>>
      stream(stub->Execute(&context));

  for (int i = 0; i < FLAGS_repetitions; i++) {
    if (!stream->Write(request)) {
      break;
    }
    api::ExecuteResponse response;
    stream->Read(&response);
    LOG(INFO) << response.DebugString();
    request.clear_container_spec();
  }
  stream->WritesDone();
  grpc::Status status = stream->Finish();
  if (!status.ok()) {
    LOG(ERROR) << status.error_message();
  }
}
