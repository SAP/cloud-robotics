FUNCTION zewm_rf_robco_0100_pbo.
*"----------------------------------------------------------------------
*"*"Local Interface:
*"  CHANGING
*"     REFERENCE(CS_ROBOT) TYPE  ZEWM_S_RF_ROBOT
*"     REFERENCE(SELECTION) TYPE  /SCWM/S_RF_SELECTION
*"----------------------------------------------------------------------

  DATA:
    ls_rsrc TYPE /scwm/rsrc.


  BREAK-POINT ID zewm_robco.

* Introduce the parameters
  CALL METHOD /scwm/cl_rf_bll_srvc=>init_screen_param.

  CALL METHOD /scwm/cl_rf_bll_srvc=>set_screen_param
    EXPORTING
      iv_param_name = 'CS_ROBOT'.

* call standard
  CALL FUNCTION '/SCWM/RF_PICK_PIBUSR_PBO'
    CHANGING
      selection = selection.

ENDFUNCTION.
