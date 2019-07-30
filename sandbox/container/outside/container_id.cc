#include "sandbox/container/outside/container_id.h"

#include <memory>
#include <numeric>

#include "glog/logging.h"
#include "util/cpp/config.h"

namespace omogen {
namespace sandbox {

ContainerIds CONTAINER_IDS(
    omogen::util::ReadConfigInt("/etc/omogen/sandbox/users.conf", "range"));

ContainerIds::ContainerIds(int limit) : containerIds(limit) {
  std::iota(containerIds.begin(), containerIds.end(), 0);
}

static bool hasIds(std::vector<int>* v) { return !v->empty(); }

std::unique_ptr<ContainerId> ContainerIds::Get() {
  mutex.LockWhen(absl::Condition(hasIds, &containerIds));
  std::unique_ptr<ContainerId> ret =
      std::make_unique<ContainerId>(containerIds.back(), this);
  LOG(INFO) << "Retreving " << ret->Get();
  containerIds.pop_back();
  mutex.Unlock();
  return std::move(ret);
}

ContainerId& ContainerId::operator=(ContainerId&& other) {
  if (this != &other) {
    if (containerIds) {
      containerIds->Put(id);
      id = other.id;
      other.containerIds = nullptr;
    }
  }
  return *this;
}

void ContainerIds::Put(int id) {
  absl::MutexLock lock(&mutex);
  containerIds.push_back(id);
}

ContainerId::~ContainerId() {
  if (containerIds != nullptr) {
    LOG(INFO) << "Returning " << id;
    containerIds->Put(id);
  }
}

std::unique_ptr<ContainerId> ContainerIds::GetId() {
  return CONTAINER_IDS.Get();
}

}  // namespace sandbox
}  // namespace omogen