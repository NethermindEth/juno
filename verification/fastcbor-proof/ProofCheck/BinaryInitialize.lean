import ProofCheck.InitializationState
import Perennial.Proof.encoding.binary.initialize
import Perennial.Proof.errors.New
import Perennial.Proof.errors.initialize
import Perennial.Proof.math.initialize
import Perennial.Proof.unsafe.initialize
import Perennial.Proof.cmp.initialize
import Perennial.Proof.math.bits.initialize
import Perennial.Proof.slices.initialize
import Perennial.Proof.internal.synctest.initialize
import Perennial.Proof.internal.race.initialize
import Perennial.Proof.sync.atomic.initialize
import Perennial.Proof.sync.initialize
import Perennial.Proof.io.initialize

set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Close the binary package's initialization dependency with the existing checked
    Perennial proofs, including sync/io and their dependencies. This introduces no
    caller-supplied initializer contract; the pinned standard-library models remain
    part of the existing semantics boundary. -/
theorem wp_binary_initialize :
    DecoderDependencyInit (GF := GF) pkg_id.encoding.binary encoding.binary.initialize' := by
  have herrors := errors.initialize'.wp_proof (GF := GF) errors.New.wp_proof
  have hsync := sync.initialize'.wp_proof (GF := GF)
    internal.synctest.initialize'.wp_proof internal.race.initialize'.wp_proof sync.atomic.initialize'.wp_proof
  have hslices := slices.initialize'.wp_proof (GF := GF)
    (math.bits.initialize'.wp_proof «unsafe».initialize'.wp_proof) cmp.initialize'.wp_proof
  exact encoding.binary.initialize'.wp_proof hsync hslices math.initialize'.wp_proof
    (io.initialize'.wp_proof hsync herrors) herrors errors.New.wp_proof

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_binary_initialize
