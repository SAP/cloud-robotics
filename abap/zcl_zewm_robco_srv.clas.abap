class ZCL_ZEWM_ROBCO_SRV definition
  public
  final
  create public .

public section.

  methods CONSTRUCTOR
    importing
      !IV_LGNUM type /SCWM/LGNUM .
  methods GET_INSTANCE
    importing
      !IV_LGNUM type /SCWM/LGNUM
    returning
      value(EO_INSTANCE) type ref to ZCL_ZEWM_ROBCO_SRV .
  class-methods WCR_IS_ROBOT
    importing
      !IV_LGNUM type /SCWM/LGNUM
      !IV_WHO type /SCWM/DE_WHO
    exporting
      !EV_TOP_WHO type /SCWM/DE_WHO
      value(EV_IS_ROBOT) type BOOLE_D
      !EV_ROBOT_RSRC type /SCWM/DE_RSRC .
protected section.

  data MT_RSRC type /SCWM/TT_RSRC .
  data MT_RSRC_TYPE type /SCWM/TT_TRSRC_TYP .
  data MO_INSTANCE type ref to ZCL_ZEWM_ROBCO_SRV .
private section.
ENDCLASS.



CLASS ZCL_ZEWM_ROBCO_SRV IMPLEMENTATION.


  METHOD constructor.

*  get resource types




  ENDMETHOD.


  METHOD get_instance.

    IF mo_instance IS NOT BOUND.
      eo_instance = mo_instance.
    ELSE.
      CREATE OBJECT mo_instance
        EXPORTING
          iv_lgnum = iv_lgnum.
    ENDIF.

  ENDMETHOD.


  METHOD wcr_is_robot.

    DATA: lv_resource TYPE /scwm/de_rsrc,
          ls_twcr     TYPE /scwm/twcr,
          ls_who      TYPE /scwm/s_who_int,
          ls_who_top  TYPE /scwm/s_who_int.


    BREAK-POINT ID zewm_robco.

    CLEAR: ev_is_robot, ev_top_who, ev_robot_rsrc.
*   get who
    CALL FUNCTION '/SCWM/WHO_GET'
      EXPORTING
        iv_lgnum = iv_lgnum
        iv_whoid = iv_who
      IMPORTING
        es_who   = ls_who.

*   check wcr of who
    CALL FUNCTION '/SCWM/TWCR_READ_SINGLE'
      EXPORTING
        iv_lgnum  = iv_lgnum
        iv_wcr    = ls_who-wcr
      IMPORTING
        es_twcr   = ls_twcr
      EXCEPTIONS
        not_found = 1
        OTHERS    = 2.
    IF sy-subrc <> 0.
*     Implement suitable error handling here
    ENDIF.

    IF ls_twcr-zz_wcr_robot EQ abap_true.
*     RobCo scenario > get resource of top-who
      CALL FUNCTION '/SCWM/WHO_GET'
        EXPORTING
          iv_lgnum = iv_lgnum
          iv_whoid = ls_who-topwhoid
        IMPORTING
          es_who   = ls_who_top.

*     set robot as resource
      ev_is_robot = abap_true.
      ev_top_who = ls_who_top-who.
      ev_robot_rsrc = ls_who_top-rsrc.
    ENDIF.

  ENDMETHOD.
ENDCLASS.
