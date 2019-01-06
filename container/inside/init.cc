#include <sys/prctl.h>
#include <sys/resource.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>
#include <cstdio>
#include <iostream>
#include <string>

#include "container/inside/chroot.h"
#include "container/inside/setup.h"
#include "glog/logging.h"
#include "glog/raw_logging.h"
#include "proto/container.pb.h"
#include "util/files.h"

namespace omogenexec {

// Returns a termination cause based on the given status, which should
// be given in the waitpid format.
static proto::ContainerTermination terminationForStatus(int waitStatus) {
  proto::ContainerTermination termination;
  if (WIFEXITED(waitStatus)) {
    termination.mutable_exit()->set_code(WEXITSTATUS(waitStatus));
  } else if (WIFSIGNALED(waitStatus)) {
    termination.mutable_signal()->set_signal(WTERMSIG(waitStatus));
  } else {
    assert(false && "Invalid exit status");
  }
  return termination;
}

// Returns a termination for a sandbox-level error that occured.
static proto::ContainerTermination terminationForError(
    const std::string& errorMessage) {
  proto::ContainerTermination termination;
  termination.mutable_error()->set_error_message(errorMessage);
  return termination;
}

static proto::ContainerTermination execute(
    const proto::ContainerExecution& request) {
  // We need to set this here rather than in setup since we lose privilege to
  // change this to a potentially higher number after the fork.
  rlim_t processLimit = request.process_limit() + 1;  // +1 for init
  rlimit rlim = {.rlim_cur = processLimit, processLimit};
  PCHECK(setrlimit(RLIMIT_NPROC, &rlim) != -1)
      << "Could not set the process limit";

  // Start a fork to set up the execution environment for the request.
  // In the parent, we will wait for the request to finish. Since an error
  // may occur during setup of the contained process, we keep a pipe open
  // that the child can send error messages to us with.
  int errorPipe[2];
  PCHECK(pipe(errorPipe) != -1) << "Could not create error pipe";

  // Note that the child process will not survive the parent death since the
  // parent is the init process of a new PID namespace. That means the child is
  // automatically killed if the parent is killed.
  pid_t which = fork();
  if (which == 0) {
    VLOG(2) << "Container child started";
    close(errorPipe[0]);                 // We only write to the error pipe
    SetupAndRun(request, errorPipe[1]);  // Will either execve or crash
  } else {
    PCHECK(which != -1) << "Could not fork execution process";
    close(errorPipe[1]);  // We only read from the error pipe
    // Try reading an error message until the pipe closes. The child writes
    // a \1 byte just before it decides to close the pipe. This means that we
    // can also be aware of errors that crashes the child (without first being
    // handled by sending us an error message) by checking if we get a \1 byte.
    // This will never we part of a normal error message, since they only
    // contain printable ASCII.
    std::string errorMessage;
    char buf[1024];
    while (true) {
      int ret = read(errorPipe[0], buf, sizeof(buf));
      PCHECK(ret != -1 || errno == EINTR) << "Could not read error message";
      // We were interrupted.
      if (ret == -1) {
        continue;
      }
      // Pipe was closed.
      if (ret == 0) {
        break;
      }
      errorMessage += std::string(buf, buf + ret);
    }
    // The process crashed before it closed its error stream.
    if (errorMessage.empty()) {
      return terminationForError("Unhandled error during setup");
    }
    if (errorMessage[0] != '\1') {
      return terminationForError("Error during execution setup: " +
                                 errorMessage);
    }
    while (true) {
      int status;
      int ret = waitpid(which, &status, 0);
      PCHECK(ret != -1 || errno == EINTR) << "Could not wait";
      // We were interrupted
      if (ret == -1) {
        continue;
      }
      // If our child process decided to start children of their own, they will
      // now become a child of us since we are init. Therefore, we make sure to
      // SIGKILL all of them before we return, in case we want to reuse our
      // sandbox.
      PCHECK(kill(-1, SIGKILL) != -1)
          << "Did not manage to kill all remaining processes in the container";
      while (true) {
        int ret = waitpid(-1, nullptr, WNOHANG);
        PCHECK(ret != -1) << "Could not wait for remaining daemons";
        if (ret == 0) {
          break;
        }
      }
      return terminationForStatus(status);
    }
  }
}

}  // namespace omogenexec

int main(int argc, char** argv) {
  gflags::ParseCommandLineFlags(&argc, &argv, true);
  google::InitGoogleLogging(argv[0]);
  // Kill us if the main sandbox is killed, to prevent our child from possibly
  // keeping running. This is not a race with the parent death, since the read
  // later will crash us in case our parent dies after the prctl call.
  CHECK(prctl(PR_SET_PDEATHSIG, SIGTERM) != -1)
      << "Could not set PR_SET_PDEATHSIG";
  LOG(INFO) << "Started up container with pid " << getpid();
  // Keep reading execution requests in a loop in case we want to run more
  // commands in the same sandbox.
  while (true) {
    // Read execution request from the parent.
    omogenexec::proto::ContainerExecution request;
    LOG(INFO) << "Waiting for request...";
    int length;
    if (!omogenexec::ReadIntFromFd(&length, 0)) {
      break;
    }
    std::string requestBytes = omogenexec::ReadFromFd(length, 0);
    if (!request.ParseFromString(requestBytes)) {
      LOG(ERROR) << "Could not read complete request";
    }
    LOG(INFO) << "Received request " << request.DebugString();
    omogenexec::proto::ContainerTermination response =
        omogenexec::execute(request);
    LOG(INFO) << "Done with termination " << response.DebugString();
    std::string responseBytes;
    response.SerializeToString(&responseBytes);
    LOG(INFO) << "Trying to write int " << responseBytes.size();
    omogenexec::WriteIntToFd(responseBytes.size(), 6);
    omogenexec::WriteToFd(6, responseBytes);
    LOG(INFO) << "Written response";
  }
  PCHECK(close(6) != -1) << "Could not close stdout";
  gflags::ShutDownCommandLineFlags();
}
