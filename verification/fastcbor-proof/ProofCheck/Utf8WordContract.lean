import ProofCheck.Utf8Scan

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics]

/-- The eight-byte helper contract at the actual generic function
    instantiated by unicode/utf8.Valid. Callers must supply this explicit
    proposition; it is neither an axiom nor an instance. Utf8WordImpl proves it
    against the pinned upstream indexing fix. The whole input remains owned
    while the helper reads its window. -/
def Utf8WordWindowContract : Prop :=
  ∀ (data : GoSlice) (xs : List w8) (off : Nat) (dq : DFrac), off + 8 ≤ xs.length →
    {{ isPkgInit (PROP := IProp GF) pkg_id.unicode.utf8 ∗ data ↦*{dq} xs }}
      (App (Val #(functions unicode.utf8.word [go.SliceType go.byte]))
        (Val #(utf8Window data (W64 off))))
    {{ RET #(leToU64 ((xs.drop off).take 8)); data ↦*{dq} xs }}

end github_com.NethermindEth.juno.core
end Perennial
