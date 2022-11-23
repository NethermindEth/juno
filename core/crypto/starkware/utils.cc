#include <algorithm>
#include <cstring>

#include "utils.h"
#include "portable_endian.h"

namespace starkware {

using ValueType = PrimeFieldElement::ValueType;

int HandleError(const char* msg, gsl::span<gsl::byte> out) {
  const size_t copy_len = std::min<size_t>(strlen(msg), out.size() - 1);
  memcpy(out.data(), msg, copy_len);
  memset(out.data() + copy_len, 0, out.size() - copy_len);
  return 1;
}

ValueType Deserialize(const gsl::span<const gsl::byte> span, bool le) {
  const size_t N = ValueType::LimbCount();
  ASSERT(span.size() == N * sizeof(uint64_t), "Source span size mismatches BigInt size.");
  std::array<uint64_t, N> value{};
  gsl::copy(span, gsl::byte_span(value));
  for (uint64_t& x : value) {
      x = le ? le64toh(x) : be64toh(x);
  }
  if (!le)
    std::reverse(std::begin(value), std::end(value));
  return ValueType(value);
}

void Serialize(const ValueType& val, const gsl::span<gsl::byte> span_out, bool le) {
  const size_t N = ValueType::LimbCount();
  ASSERT(span_out.size() == N * sizeof(uint64_t), "Span size mismatches BigInt size.");
  for (size_t i = 0; i < N; ++i) {
    uint64_t limb = le ? htole64(val[i]) : htobe64(val[i]);
    size_t idx = le ? i : N - i - 1;
    gsl::copy(gsl::byte_span(limb), span_out.subspan(idx * sizeof(uint64_t), sizeof(uint64_t)));
  }
}

}  // namespace starkware
