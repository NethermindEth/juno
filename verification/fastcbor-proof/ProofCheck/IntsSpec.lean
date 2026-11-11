import ProofCheck.Int64Complete

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Valid index elements retain their wire width and specify their signed value.
    Null/undefined elements are zero, unlike a null array, which yields nil. -/
inductive IntElementWire where
  | nullish (head : w8) (nullHead : head = W8 246 ∨ head = W8 247)
  | unsigned (head : w8) (payload : List w8) (argument : w64)
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0)
      (bound : uint.Z argument < 2 ^ 63)
  | negative (head : w8) (payload : List w8) (argument : w64)
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 32)
      (bound : uint.Z argument < 2 ^ 63)

def IntElementWire.bytes : IntElementWire → List w8
  | .nullish head _ => [head]
  | .unsigned head payload _ _ _ _ | .negative head payload _ _ _ _ => head :: payload

def IntElementWire.value : IntElementWire → w64
  | .nullish _ _ => W64 0
  | .unsigned _ _ argument _ _ _ => argument
  | .negative _ _ argument _ _ _ => ~~~argument

def IntElementWire.number : IntElementWire → Int
  | .nullish _ _ => 0
  | .unsigned _ _ argument _ _ _ => uint.Z argument
  | .negative _ _ argument _ _ _ => -1 - uint.Z argument

theorem intElementWire_value (item : IntElementWire) : sint.Z item.value = item.number := by
  cases item <;> simp only [IntElementWire.value, IntElementWire.number] <;> word

theorem intElementWire_nonempty (item : IntElementWire) : 0 < item.bytes.length := by
  cases item <;> simp [IntElementWire.bytes]

theorem intElementWire_count_bound (items : List IntElementWire) :
    items.length ≤ (items.flatMap IntElementWire.bytes).length := by
  induction items with
  | nil => simp
  | cons item items ih =>
    have := intElementWire_nonempty item
    simp only [List.length_cons, List.flatMap_cons, List.length_append]; omega

/-- Independent nullable-array acceptance, including the container depth bound. -/
inductive IntsValid : Nat → List w8 → Option (List Int) → Prop where
  | nullish {d : Nat} {head : w8} (hn : head = W8 246 ∨ head = W8 247) :
      IntsValid d [head] none
  | array {d : Nat} {head : w8} {payload : List w8} {count : w64}
      (items : List IntElementWire) (enc : HeadArgument head payload count)
      (major : head &&& W8 224 = W8 128) (size : uint.Z count = items.length) (bound : d < 32) :
      IntsValid d ((head :: payload) ++ items.flatMap IntElementWire.bytes) (some (items.map IntElementWire.number))

def IntsResult (out : GoSlice) (value : Option (List Int)) : IProp GF :=
  match value with
  | none => iprop(⌜out = slice.nil⌝)
  | some values => iprop(∃ words : List w64, out ↦* words ∗ ⌜out.ptr ≠ null ∧ words.map sint.Z = values⌝)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.intElementWire_value
#print axioms Perennial.github_com.NethermindEth.juno.core.intElementWire_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.intElementWire_count_bound
