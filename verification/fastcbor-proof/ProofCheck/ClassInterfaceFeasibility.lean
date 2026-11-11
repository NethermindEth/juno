import ProofCheck.GeneratedProof.github_com.NethermindEth.juno.verification.fastcbor_proof.feasibility.interfaces

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics]
variable [package_sem : github_com.NethermindEth.juno.verification.fastcbor_proof.feasibility.interfaces.Assumptions]

open github_com.NethermindEth.juno.verification.fastcbor_proof.feasibility.interfaces

/-- Actual pointer-to-ClassDefinition conversion, including a typed nil pointer.
    The payload is not accessed and no class method is invoked. -/
theorem wp_boxCairo_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! BoxCairo)) (Val #p))
    {{ RET #(interface.mkOk (go.PointerType DeprecatedCairoClass.ty) #p); True }} := by
  wp_start
  wp_auto
  wp_end

/-- Concrete pointer classification preserves the distinction between typed nil and nil. -/
theorem wp_classifyCairo_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! ClassifyClass))
        (Val #(interface.mkOk (go.PointerType DeprecatedCairoClass.ty) #p)))
    {{ RET (PairV #(W8 1) #(decide (p = null))); True }} := by
  wp_start
  wp_auto
  wp_end

/-- Actual pointer-to-ClassDefinition conversion, including a typed nil pointer.
    The payload is not accessed and no class method is invoked. -/
theorem wp_boxSierra_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! BoxSierra)) (Val #p))
    {{ RET #(interface.mkOk (go.PointerType SierraClass.ty) #p); True }} := by
  wp_start
  wp_auto
  wp_end

/-- Concrete pointer classification preserves the distinction between typed nil and nil. -/
theorem wp_classifySierra_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! ClassifyClass))
        (Val #(interface.mkOk (go.PointerType SierraClass.ty) #p)))
    {{ RET (PairV #(W8 2) #(decide (p = null))); True }} := by
  wp_start
  wp_auto
  wp_end

/-- A nil class interface follows the nil branch. -/
theorem wp_classifyClass_nil_witness :
    {{ (True : IProp GF) }}
      (App (Val (@! ClassifyClass)) (Val #interface.nil))
    {{ RET (PairV #(W8 0) #false); True }} := by
  wp_start
  wp_auto
  wp_end

/-- Unrecognized dynamic types reach default without payload access or method calls. -/
theorem wp_classifyOtherClass_witness (t : go.GoType) (v : val)
    (hcairo : t ≠ go.PointerType DeprecatedCairoClass.ty)
    (hsierra : t ≠ go.PointerType SierraClass.ty) :
    {{ (True : IProp GF) }}
      (App (Val (@! ClassifyClass)) (Val #(interface.mkOk t v)))
    {{ RET (PairV #(W8 3) #false); True }} := by
  wp_start
  wp_auto
  simp only [hcairo, ite_false, decide_false]
  wp_auto
  simp only [hsierra, ite_false, decide_false]
  wp_auto
  wp_end

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_boxCairo_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_classifyCairo_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_boxSierra_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_classifySierra_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_classifyClass_nil_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_classifyOtherClass_witness
