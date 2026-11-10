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

/-- Actual pointer-to-Transaction conversion, preserving the pointer including nil.
    The pointed-to transaction payload is not accessed. -/
theorem wp_boxDeploy_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! BoxDeploy)) (Val #p))
    {{ RET #(interface.mkOk (go.PointerType DeployTransaction.ty) #p); True }} := by
  wp_start
  wp_auto
  wp_end

/-- Actual pointer-to-Transaction conversion, preserving the pointer including nil.
    The pointed-to transaction payload is not accessed. -/
theorem wp_boxDeployAccount_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! BoxDeployAccount)) (Val #p))
    {{ RET #(interface.mkOk (go.PointerType DeployAccountTransaction.ty) #p); True }} := by
  wp_start
  wp_auto
  wp_end

/-- Actual pointer-to-Transaction conversion, preserving the pointer including nil.
    The pointed-to transaction payload is not accessed. -/
theorem wp_boxInvoke_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! BoxInvoke)) (Val #p))
    {{ RET #(interface.mkOk (go.PointerType InvokeTransaction.ty) #p); True }} := by
  wp_start
  wp_auto
  wp_end

/-- Actual pointer-to-Transaction conversion, preserving the pointer including nil.
    The pointed-to transaction payload is not accessed. -/
theorem wp_boxDeclare_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! BoxDeclare)) (Val #p))
    {{ RET #(interface.mkOk (go.PointerType DeclareTransaction.ty) #p); True }} := by
  wp_start
  wp_auto
  wp_end

/-- Actual pointer-to-Transaction conversion, preserving the pointer including nil.
    The pointed-to transaction payload is not accessed. -/
theorem wp_boxL1Handler_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! BoxL1Handler)) (Val #p))
    {{ RET #(interface.mkOk (go.PointerType L1HandlerTransaction.ty) #p); True }} := by
  wp_start
  wp_auto
  wp_end

/-- A nil interface follows the nil case, independently of typed-nil pointers. -/
theorem wp_classifyTransaction_nil_witness :
    {{ (True : IProp GF) }}
      (App (Val (@! ClassifyTransaction)) (Val #interface.nil))
    {{ RET (PairV #(W8 0) #false); True }} := by
  wp_start
  wp_auto
  wp_end

/-- The actual type switch recognizes this dynamic pointer type, including nil. -/
theorem wp_classifyDeploy_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! ClassifyTransaction))
        (Val #(interface.mkOk (go.PointerType DeployTransaction.ty) #p)))
    {{ RET (PairV #(W8 1) #(decide (p = null))); True }} := by
  wp_start
  wp_auto
  wp_end

/-- The actual type switch recognizes this dynamic pointer type, including nil. -/
theorem wp_classifyDeployAccount_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! ClassifyTransaction))
        (Val #(interface.mkOk (go.PointerType DeployAccountTransaction.ty) #p)))
    {{ RET (PairV #(W8 2) #(decide (p = null))); True }} := by
  wp_start
  wp_auto
  wp_end

/-- The actual type switch recognizes this dynamic pointer type, including nil. -/
theorem wp_classifyInvoke_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! ClassifyTransaction))
        (Val #(interface.mkOk (go.PointerType InvokeTransaction.ty) #p)))
    {{ RET (PairV #(W8 3) #(decide (p = null))); True }} := by
  wp_start
  wp_auto
  wp_end

/-- The actual type switch recognizes this dynamic pointer type, including nil. -/
theorem wp_classifyDeclare_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! ClassifyTransaction))
        (Val #(interface.mkOk (go.PointerType DeclareTransaction.ty) #p)))
    {{ RET (PairV #(W8 4) #(decide (p = null))); True }} := by
  wp_start
  wp_auto
  wp_end

/-- The actual type switch recognizes this dynamic pointer type, including nil. -/
theorem wp_classifyL1Handler_witness (p : Loc) :
    {{ (True : IProp GF) }}
      (App (Val (@! ClassifyTransaction))
        (Val #(interface.mkOk (go.PointerType L1HandlerTransaction.ty) #p)))
    {{ RET (PairV #(W8 5) #(decide (p = null))); True }} := by
  wp_start
  wp_auto
  wp_end

/-- Other dynamic types reach default without reading their value or calling methods. -/
theorem wp_classifyOtherTransaction_witness (t : go.GoType) (v : val)
    (hdeploy : t ≠ go.PointerType DeployTransaction.ty)
    (haccount : t ≠ go.PointerType DeployAccountTransaction.ty)
    (hinvoke : t ≠ go.PointerType InvokeTransaction.ty)
    (hdeclare : t ≠ go.PointerType DeclareTransaction.ty)
    (hhandler : t ≠ go.PointerType L1HandlerTransaction.ty) :
    {{ (True : IProp GF) }}
      (App (Val (@! ClassifyTransaction)) (Val #(interface.mkOk t v)))
    {{ RET (PairV #(W8 6) #false); True }} := by
  wp_start
  wp_auto
  simp only [hdeploy, ite_false, decide_false]
  wp_auto
  simp only [haccount, ite_false, decide_false]
  wp_auto
  simp only [hinvoke, ite_false, decide_false]
  wp_auto
  simp only [hdeclare, ite_false, decide_false]
  wp_auto
  simp only [hhandler, ite_false, decide_false]
  wp_auto
  wp_end

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_boxDeploy_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_boxDeployAccount_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_boxInvoke_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_boxDeclare_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_boxL1Handler_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_classifyTransaction_nil_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_classifyDeploy_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_classifyDeployAccount_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_classifyInvoke_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_classifyDeclare_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_classifyL1Handler_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_classifyOtherTransaction_witness
