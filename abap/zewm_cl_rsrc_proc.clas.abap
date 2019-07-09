class ZEWM_CL_RSRC_PROC definition
  public
  final
  create public .

public section.

  interfaces /SCWM/IF_EX_RSRC_PROC_WO .
  interfaces IF_BADI_INTERFACE .
protected section.
private section.
ENDCLASS.



CLASS ZEWM_CL_RSRC_PROC IMPLEMENTATION.


  method /SCWM/IF_EX_RSRC_PROC_WO~LSD_PRIO_UPDATE.
    if 1 = 2.
      BREAK-POINT.
    endif.



  endmethod.
ENDCLASS.
