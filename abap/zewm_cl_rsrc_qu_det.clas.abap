class ZEWM_CL_RSRC_QU_DET definition
  public
  final
  create public .

public section.

  interfaces /SCWM/IF_EX_RSRC_QU_DET .
  interfaces IF_BADI_INTERFACE .
protected section.
private section.
ENDCLASS.



CLASS ZEWM_CL_RSRC_QU_DET IMPLEMENTATION.


  method /SCWM/IF_EX_RSRC_QU_DET~DETERMINE.
    if 1 = 2.
      BREAK-POINT.
    endif.
  endmethod.
ENDCLASS.
