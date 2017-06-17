#ifndef PUFFS_GIF_H
#define PUFFS_GIF_H

// Code generated by puffs-gen-c. DO NOT EDIT.

#ifndef PUFFS_BASE_HEADER_H
#define PUFFS_BASE_HEADER_H

// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

#include <stdbool.h>
#include <stdint.h>
#include <string.h>

// Puffs requires a word size of at least 32 bits because it assumes that
// converting a u32 to usize will never overflow. For example, the size of a
// decoded image is often represented, explicitly or implicitly in an image
// file, as a u32, and it is convenient to compare that to a buffer size.
//
// Similarly, the word size is at most 64 bits because it assumes that
// converting a usize to u64 will never overflow.
#if __WORDSIZE < 32
#error "Puffs requires a word size of at least 32 bits"
#elif __WORDSIZE > 64
#error "Puffs requires a word size of at most 64 bits"
#endif

// PUFFS_VERSION is the major.minor version number as a uint32. The major
// number is the high 16 bits. The minor number is the low 16 bits.
//
// The intention is to bump the version number at least on every API / ABI
// backwards incompatible change.
//
// For now, the API and ABI are simply unstable and can change at any time.
//
// TODO: don't hard code this in base-header.h.
#define PUFFS_VERSION (0x00001)

// puffs_base_buf1 is a 1-dimensional buffer (a pointer and length) plus
// additional indexes into that buffer.
//
// A value with all fields NULL or zero is a valid, empty buffer.
typedef struct {
  uint8_t* ptr;  // Pointer.
  size_t len;    // Length.
  size_t wi;     // Write index. Invariant: wi <= len.
  size_t ri;     // Read  index. Invariant: ri <= wi.
  bool closed;   // No further writes are expected.
} puffs_base_buf1;

#endif  // PUFFS_BASE_HEADER_H

#ifdef __cplusplus
extern "C" {
#endif

// ---------------- Status Codes

// Status codes are non-positive integers.
//
// The least significant bit indicates a non-recoverable status code: an error.
typedef enum {
  puffs_gif_status_ok = 0,
  puffs_gif_error_bad_version = -2 + 1,
  puffs_gif_error_bad_receiver = -4 + 1,
  puffs_gif_error_bad_argument = -6 + 1,
  puffs_gif_error_constructor_not_called = -8 + 1,
  puffs_gif_error_unexpected_eof = -10 + 1,
  puffs_gif_status_short_read = -12,
  puffs_gif_status_short_write = -14,
  puffs_gif_error_closed_for_writes = -16 + 1,
  puffs_gif_error_bad_gif_header = -256 + 1,
  puffs_gif_error_lzw_code_is_out_of_range = -258 + 1,
  puffs_gif_error_lzw_prefix_chain_is_cyclical = -260 + 1,
} puffs_gif_status;

bool puffs_gif_status_is_error(puffs_gif_status s);

const char* puffs_gif_status_string(puffs_gif_status s);

// ---------------- Structs

typedef struct {
  // Do not access the private_impl's fields directly. There is no API/ABI
  // compatibility or safety guarantee if you do so. Instead, use the
  // puffs_gif_lzw_decoder_etc functions.
  //
  // In C++, these fields would be "private", but C does not support that.
  //
  // It is a struct, not a struct*, so that it can be stack allocated.
  struct {
    puffs_gif_status status;
    uint32_t magic;
    uint32_t f_literal_width;
    uint8_t f_stack[4096];
    uint8_t f_suffixes[4096];
    uint16_t f_prefixes[4096];
  } private_impl;
} puffs_gif_lzw_decoder;

typedef struct {
  // Do not access the private_impl's fields directly. There is no API/ABI
  // compatibility or safety guarantee if you do so. Instead, use the
  // puffs_gif_decoder_etc functions.
  //
  // In C++, these fields would be "private", but C does not support that.
  //
  // It is a struct, not a struct*, so that it can be stack allocated.
  struct {
    puffs_gif_status status;
    uint32_t magic;
    puffs_gif_lzw_decoder f_lzw;
  } private_impl;
} puffs_gif_decoder;

// ---------------- Public Constructor and Destructor Prototypes

// puffs_gif_lzw_decoder_constructor is a constructor function.
//
// It should be called before any other puffs_gif_lzw_decoder_* function.
//
// Pass PUFFS_VERSION and 0 for puffs_version and for_internal_use_only.
void puffs_gif_lzw_decoder_constructor(puffs_gif_lzw_decoder* self,
                                       uint32_t puffs_version,
                                       uint32_t for_internal_use_only);

void puffs_gif_lzw_decoder_destructor(puffs_gif_lzw_decoder* self);

// puffs_gif_decoder_constructor is a constructor function.
//
// It should be called before any other puffs_gif_decoder_* function.
//
// Pass PUFFS_VERSION and 0 for puffs_version and for_internal_use_only.
void puffs_gif_decoder_constructor(puffs_gif_decoder* self,
                                   uint32_t puffs_version,
                                   uint32_t for_internal_use_only);

void puffs_gif_decoder_destructor(puffs_gif_decoder* self);

// ---------------- Public Function Prototypes

puffs_gif_status puffs_gif_decoder_decode(puffs_gif_decoder* self,
                                          puffs_base_buf1* a_dst,
                                          puffs_base_buf1* a_src);

void puffs_gif_lzw_decoder_set_literal_width(puffs_gif_lzw_decoder* self,
                                             uint32_t a_lw);

puffs_gif_status puffs_gif_lzw_decoder_decode(puffs_gif_lzw_decoder* self,
                                              puffs_base_buf1* a_dst,
                                              puffs_base_buf1* a_src);

#ifdef __cplusplus
}  // extern "C"
#endif

#endif  // PUFFS_GIF_H

// C HEADER ENDS HERE.

#ifndef PUFFS_BASE_IMPL_H
#define PUFFS_BASE_IMPL_H

#define PUFFS_LOW_BITS(x, n) ((x) & ((1 << (n)) - 1))

#endif  // PUFFS_BASE_IMPL_H

// ---------------- Status Codes Implementations

bool puffs_gif_status_is_error(puffs_gif_status s) {
  return s & 1;
}

const char* puffs_gif_status_strings[12] = {
    "gif: ok",
    "gif: bad version",
    "gif: bad receiver",
    "gif: bad argument",
    "gif: constructor not called",
    "gif: unexpected EOF",
    "gif: short read",
    "gif: short write",
    "gif: closed for writes",
    "gif: bad GIF header",
    "gif: LZW code is out of range",
    "gif: LZW prefix chain is cyclical",
};

const char* puffs_gif_status_string(puffs_gif_status s) {
  s = -(s >> 1);
  if (0 <= s) {
    if (s < 9) {
      return puffs_gif_status_strings[s];
    }
    s -= 119;
    if ((9 <= s) && (s < 12)) {
      return puffs_gif_status_strings[s];
    }
  }
  return "gif: unknown status";
}

// ---------------- Private Constructor and Destructor Prototypes

// ---------------- Private Function Prototypes

puffs_gif_status puffs_gif_decoder_decode_header(puffs_gif_decoder* self,
                                                 puffs_base_buf1* a_src);

puffs_gif_status puffs_gif_decoder_decode_lsd(puffs_gif_decoder* self,
                                              puffs_base_buf1* a_src);

// ---------------- Constructor and Destructor Implementations

// PUFFS_MAGIC is a magic number to check that constructors are called. It's
// not foolproof, given C doesn't automatically zero memory before use, but it
// should catch 99.99% of cases.
//
// Its (non-zero) value is arbitrary, based on md5sum("puffs").
#define PUFFS_MAGIC (0xCB3699CCU)

// PUFFS_ALREADY_ZEROED is passed from a container struct's constructor to a
// containee struct's constructor when the container has already zeroed the
// containee's memory.
//
// Its (non-zero) value is arbitrary, based on md5sum("zeroed").
#define PUFFS_ALREADY_ZEROED (0x68602EF1U)

void puffs_gif_lzw_decoder_constructor(puffs_gif_lzw_decoder* self,
                                       uint32_t puffs_version,
                                       uint32_t for_internal_use_only) {
  if (!self) {
    return;
  }
  if (puffs_version != PUFFS_VERSION) {
    self->private_impl.status = puffs_gif_error_bad_version;
    return;
  }
  if (for_internal_use_only != PUFFS_ALREADY_ZEROED) {
    memset(self, 0, sizeof(*self));
  }
  self->private_impl.magic = PUFFS_MAGIC;
  self->private_impl.f_literal_width = 8;
}

void puffs_gif_lzw_decoder_destructor(puffs_gif_lzw_decoder* self) {
  if (!self) {
    return;
  }
}

void puffs_gif_decoder_constructor(puffs_gif_decoder* self,
                                   uint32_t puffs_version,
                                   uint32_t for_internal_use_only) {
  if (!self) {
    return;
  }
  if (puffs_version != PUFFS_VERSION) {
    self->private_impl.status = puffs_gif_error_bad_version;
    return;
  }
  if (for_internal_use_only != PUFFS_ALREADY_ZEROED) {
    memset(self, 0, sizeof(*self));
  }
  self->private_impl.magic = PUFFS_MAGIC;
  puffs_gif_lzw_decoder_constructor(&self->private_impl.f_lzw, PUFFS_VERSION,
                                    PUFFS_ALREADY_ZEROED);
}

void puffs_gif_decoder_destructor(puffs_gif_decoder* self) {
  if (!self) {
    return;
  }
  puffs_gif_lzw_decoder_destructor(&self->private_impl.f_lzw);
}

// ---------------- Function Implementations

puffs_gif_status puffs_gif_decoder_decode(puffs_gif_decoder* self,
                                          puffs_base_buf1* a_dst,
                                          puffs_base_buf1* a_src) {
  if (!self) {
    return puffs_gif_error_bad_receiver;
  }
  puffs_gif_status status = self->private_impl.status;
  if (status & 1) {
    return status;
  }
  if (self->private_impl.magic != PUFFS_MAGIC) {
    status = puffs_gif_error_constructor_not_called;
    goto cleanup0;
  }
  if (!a_dst || !a_src) {
    status = puffs_gif_error_bad_argument;
    goto cleanup0;
  }

  status = puffs_gif_decoder_decode_header(self, a_src);
  if (status) {
    goto cleanup0;
  }
  status = puffs_gif_decoder_decode_lsd(self, a_src);
  if (status) {
    goto cleanup0;
  }

cleanup0:
  self->private_impl.status = status;
  return status;
}

puffs_gif_status puffs_gif_decoder_decode_header(puffs_gif_decoder* self,
                                                 puffs_base_buf1* a_src) {
  puffs_gif_status status = self->private_impl.status;

  uint8_t v_c[6];
  uint8_t v_i;

  for (size_t i = 0; i < 6; i++) {
    v_c[i] = 0;
  };
  v_i = 0;
  while (v_i < 6) {
    if (a_src->ri >= a_src->wi) {
      status = a_src->closed ? puffs_gif_error_unexpected_eof
                             : puffs_gif_status_short_read;
      return status;
    }
    uint8_t t_0 = a_src->ptr[a_src->ri++];
    v_c[v_i] = t_0;
    v_i += 1;
  }
  if ((v_c[0] != 71) || (v_c[1] != 73) || (v_c[2] != 70) || (v_c[3] != 56) ||
      ((v_c[4] != 55) && (v_c[4] != 57)) || (v_c[5] != 97)) {
    return puffs_gif_error_bad_gif_header;
  }

  return status;
}

puffs_gif_status puffs_gif_decoder_decode_lsd(puffs_gif_decoder* self,
                                              puffs_base_buf1* a_src) {
  puffs_gif_status status = self->private_impl.status;

  return status;
}

void puffs_gif_lzw_decoder_set_literal_width(puffs_gif_lzw_decoder* self,
                                             uint32_t a_lw) {
  if (!self) {
    return;
  }
  if (self->private_impl.status & 1) {
    return;
  }
  if (self->private_impl.magic != PUFFS_MAGIC) {
    self->private_impl.status = puffs_gif_error_constructor_not_called;
    return;
  }
  if (a_lw < 2 || a_lw > 8) {
    self->private_impl.status = puffs_gif_error_bad_argument;
    return;
  }

  self->private_impl.f_literal_width = a_lw;
}

puffs_gif_status puffs_gif_lzw_decoder_decode(puffs_gif_lzw_decoder* self,
                                              puffs_base_buf1* a_dst,
                                              puffs_base_buf1* a_src) {
  if (!self) {
    return puffs_gif_error_bad_receiver;
  }
  puffs_gif_status status = self->private_impl.status;
  if (status & 1) {
    return status;
  }
  if (self->private_impl.magic != PUFFS_MAGIC) {
    status = puffs_gif_error_constructor_not_called;
    goto cleanup0;
  }
  if (!a_dst || !a_src) {
    status = puffs_gif_error_bad_argument;
    goto cleanup0;
  }

  uint32_t v_clear_code;
  uint32_t v_end_code;
  bool v_use_save_code;
  uint32_t v_save_code;
  uint32_t v_prev_code;
  uint32_t v_width;
  uint32_t v_bits;
  uint32_t v_n_bits;
  uint32_t v_code;
  uint32_t v_s;
  uint32_t v_c;

  v_clear_code = (((uint32_t)(1)) << self->private_impl.f_literal_width);
  v_end_code = (v_clear_code + 1);
  v_use_save_code = 0;
  v_save_code = v_end_code;
  v_prev_code = 0;
  v_width = (self->private_impl.f_literal_width + 1);
  v_bits = 0;
  v_n_bits = 0;
label_0_continue:;
  while (true) {
    while (v_n_bits < v_width) {
      if (a_src->ri >= a_src->wi) {
        status = a_src->closed ? puffs_gif_error_unexpected_eof
                               : puffs_gif_status_short_read;
        goto cleanup0;
      }
      uint8_t t_0 = a_src->ptr[a_src->ri++];
      v_bits |= (((uint32_t)(t_0)) << v_n_bits);
      v_n_bits += 8;
    }
    v_code = PUFFS_LOW_BITS(v_bits, v_width);
    v_bits >>= v_width;
    v_n_bits -= v_width;
    if (v_code < v_clear_code) {
      if (a_dst->wi >= a_dst->len) {
        status = puffs_gif_status_short_write;
        goto cleanup0;
      }
      a_dst->ptr[a_dst->wi++] = ((uint8_t)(v_code));
      if (v_use_save_code) {
        self->private_impl.f_suffixes[v_save_code] = ((uint8_t)(v_code));
        self->private_impl.f_prefixes[v_save_code] = ((uint16_t)(v_prev_code));
      }
    } else if (v_code == v_clear_code) {
      v_use_save_code = false;
      v_save_code = v_end_code;
      v_prev_code = 0;
      v_width = (self->private_impl.f_literal_width + 1);
      goto label_0_continue;
    } else if (v_code == v_end_code) {
      status = puffs_gif_status_ok;
      goto cleanup0;
    } else if (v_code <= v_save_code) {
      v_s = 4095;
      v_c = v_code;
      if ((v_code == v_save_code) && v_use_save_code) {
        v_s -= 1;
        v_c = v_prev_code;
      }
      while (v_c >= v_clear_code) {
        self->private_impl.f_stack[v_s] = self->private_impl.f_suffixes[v_c];
        if (v_s == 0) {
          status = puffs_gif_error_lzw_prefix_chain_is_cyclical;
          goto cleanup0;
        }
        v_s -= 1;
        v_c = ((uint32_t)(self->private_impl.f_prefixes[v_c]));
      }
      self->private_impl.f_stack[v_s] = ((uint8_t)(v_c));
      if ((v_code == v_save_code) && v_use_save_code) {
        self->private_impl.f_stack[4095] = ((uint8_t)(v_c));
      }
      if (a_dst->closed) {
        status = puffs_gif_error_closed_for_writes;
        goto cleanup0;
      }
      if ((a_dst->len - a_dst->wi) <
          (sizeof(self->private_impl.f_stack) - v_s)) {
        status = puffs_gif_status_short_write;
        goto cleanup0;
      }
      memmove(a_dst->ptr + a_dst->wi, self->private_impl.f_stack + v_s,
              sizeof(self->private_impl.f_stack) - v_s);
      a_dst->wi += sizeof(self->private_impl.f_stack) - v_s;
      if (v_use_save_code) {
        self->private_impl.f_suffixes[v_save_code] = ((uint8_t)(v_c));
        self->private_impl.f_prefixes[v_save_code] = ((uint16_t)(v_prev_code));
      }
    } else {
      status = puffs_gif_error_lzw_code_is_out_of_range;
      goto cleanup0;
    }
    v_use_save_code = (v_save_code < 4095);
    if (v_use_save_code) {
      v_save_code += 1;
      if ((v_save_code == (((uint32_t)(1)) << v_width)) && (v_width < 12)) {
        v_width += 1;
      }
    }
    v_prev_code = v_code;
  }

cleanup0:
  self->private_impl.status = status;
  return status;
}
