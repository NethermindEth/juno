import ProofCheck.InitializationState

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The actual translated Go initializer establishes both complete UTF-8 tables.
    This proves startup data, not the correctness of utf8.Valid's loops. -/
theorem wp_utf8_initialize :
    DecoderDependencyInit (GF := GF) pkg_id.unicode.utf8 unicode.utf8.initialize' := by
  intro get_is_pkg_init Hinit
  wp_start as Hown
  iapply wp_package_init (heq := Hinit.1) $$ [Hown] HΦ
  iframe Hown
  iintro Hown
  wp_auto
  wp_apply wp_GlobalAlloc (V := GoArray unicode.utf8.acceptRange 16)
    unicode.utf8.acceptRanges (go.ArrayType 16 unicode.utf8.acceptRange.ty) as Haccept
  iapply wp_fupd
  wp_apply wp_GlobalAlloc (V := GoArray w8 256)
    unicode.utf8.first (go.ArrayType 256 go.uint8) as Hfirst
  ipersist Hfirst
  ipersist Haccept
  ieval (change (globalAddr unicode.utf8.first ↦□ utf8FirstTable : IProp GF) ⊢ _) at Hfirst
  ieval (change (globalAddr unicode.utf8.acceptRanges ↦□ utf8AcceptTable : IProp GF) ⊢ _) at Haccept
  ihave #Htables : Utf8Initialized (GF := GF) $$ []
  · isimp only [Utf8Initialized]
    iframe Hfirst Haccept
  imodintro
  iframe Hown
  is_pkg_init_finish

/-- Initialized UTF-8 provides the immutable tables to validator proofs. -/
theorem utf8_initialized_tables :
    isPkgInit (PROP := IProp GF) pkg_id.unicode.utf8 ⊢ Utf8Initialized (GF := GF) := by
  exact isPkgInit_access _

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_utf8_initialize
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8_initialized_tables
