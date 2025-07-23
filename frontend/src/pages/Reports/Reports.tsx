import React, { useEffect, useState } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  message,
  Tag,
  Typography,
  Descriptions,
  Drawer,
  Row,
  Col,
  Statistic,
  Progress,
  Timeline,
  Image,
  List,
  Badge,
  DatePicker,
  Select,
  Empty,
} from 'antd';
import {
  EyeOutlined,
  ReloadOutlined,
  DownloadOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { api } from '../../services/api';
import type { TestExecution, TestReport, Project, Environment } from '../../types';
import type { ColumnsType } from 'antd/es/table';

const { Title, Text } = Typography;
const { RangePicker } = DatePicker;
const { Option } = Select;

const Reports: React.FC = () => {
  const [executions, setExecutions] = useState<TestExecution[]>([]);
  const [reports, setReports] = useState<TestReport[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [loading, setLoading] = useState(false);
  const [isDetailDrawerVisible, setIsDetailDrawerVisible] = useState(false);
  const [selectedExecution, setSelectedExecution] = useState<TestExecution | null>(null);
  const [executionLogs, setExecutionLogs] = useState<any[]>([]);
  const [executionScreenshots, setExecutionScreenshots] = useState<any[]>([]);
  const [filters, setFilters] = useState<{
    project_id?: number;
    environment_id?: number;
    status?: string;
    date_range?: any;
  }>({
    project_id: undefined,
    environment_id: undefined,
    status: undefined,
    date_range: null,
  });
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 10,
    total: 0,
  });

  useEffect(() => {
    loadExecutions();
    loadReports();
    loadInitialData();
  }, [pagination.current, pagination.pageSize, filters]);

  const loadInitialData = async () => {
    try {
      const [projectsData, environmentsData] = await Promise.all([
        api.getProjects({ page: 1, page_size: 100 }),
        api.getEnvironments(),
      ]);
      setProjects(projectsData.list);
      setEnvironments(environmentsData);
    } catch (error) {
      console.error('Failed to load initial data:', error);
    }
  };

  const loadExecutions = async () => {
    setLoading(true);
    try {
      const params: any = {
        page: pagination.current,
        page_size: pagination.pageSize,
      };

      if (filters.project_id) params.project_id = filters.project_id;
      if (filters.environment_id) params.environment_id = filters.environment_id;
      if (filters.status) params.status = filters.status;
      if (filters.date_range && filters.date_range.length === 2 && filters.date_range[0] && filters.date_range[1]) {
        params.start_date = dayjs(filters.date_range[0]).format('YYYY-MM-DD');
        params.end_date = dayjs(filters.date_range[1]).format('YYYY-MM-DD');
      }

      const response = await api.getExecutions(params);
      setExecutions(response.list);
      setPagination(prev => ({
        ...prev,
        total: response.total,
      }));
    } catch (error) {
      console.error('Failed to load executions:', error);
      message.error('获取执行记录失败');
    } finally {
      setLoading(false);
    }
  };

  const loadReports = async () => {
    try {
      const response = await api.getReports();
      setReports(response.list || []);
    } catch (error) {
      console.error('Failed to load reports:', error);
    }
  };

  const handleViewDetails = async (execution: TestExecution) => {
    try {
      const [logsResponse, screenshotsResponse] = await Promise.all([
        api.getExecutionLogs(execution.id),
        api.getExecutionScreenshots(execution.id),
      ]);
      
      setSelectedExecution(execution);
      setExecutionLogs(logsResponse.logs || []);
      setExecutionScreenshots(screenshotsResponse.screenshots || []);
      setIsDetailDrawerVisible(true);
    } catch (error) {
      console.error('Failed to load execution details:', error);
      message.error('获取执行详情失败');
    }
  };

  const handleDownloadReport = async (execution: TestExecution) => {
    try {
      // Generate and download report
      const reportData = {
        execution_id: execution.id,
        format: 'html', // or 'pdf'
      };
      const response = await api.createReport(reportData);
      message.success('报告生成成功');
      
      // Trigger download
      const link = document.createElement('a');
      link.href = response.download_url;
      link.download = `test-report-${execution.id}.html`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
    } catch (error) {
      console.error('Failed to generate report:', error);
      message.error('生成报告失败');
    }
  };

  const getStatusColor = (status: string) => {
    const colors: Record<string, string> = {
      success: 'green',
      failed: 'red',
      running: 'blue',
      pending: 'orange',
      cancelled: 'gray',
    };
    return colors[status] || 'default';
  };

  const getStatusText = (status: string) => {
    const texts: Record<string, string> = {
      success: '成功',
      failed: '失败',
      running: '运行中',
      pending: '等待中',
      cancelled: '已取消',
    };
    return texts[status] || status;
  };

  const getStatusIcon = (status: string) => {
    const icons: Record<string, React.ReactNode> = {
      success: <CheckCircleOutlined style={{ color: '#52c41a' }} />,
      failed: <CloseCircleOutlined style={{ color: '#ff4d4f' }} />,
      running: <ClockCircleOutlined style={{ color: '#1890ff' }} />,
      pending: <ClockCircleOutlined style={{ color: '#fa8c16' }} />,
    };
    return icons[status] || <ClockCircleOutlined />;
  };

  const calculateSuccessRate = (executions: TestExecution[]) => {
    if (executions.length === 0) return 0;
    const successCount = executions.filter(e => e.status === 'success').length;
    return Math.round((successCount / executions.length) * 100);
  };

  const columns: ColumnsType<TestExecution> = [
    {
      title: '执行ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
    },
    {
      title: '执行类型',
      dataIndex: 'execution_type',
      key: 'execution_type',
      width: 100,
      render: (type: string) => (
        <Tag color={type === 'test_case' ? 'blue' : 'green'}>
          {type === 'test_case' ? '测试用例' : '测试套件'}
        </Tag>
      ),
    },
    {
      title: '名称',
      key: 'name',
      width: 200,
      ellipsis: true,
      render: (_, record) => (
        <div>
          <div>{record.test_case?.name || record.test_suite?.name}</div>
          <Text type="secondary" style={{ fontSize: '12px' }}>
            {record.test_case?.project?.name || record.test_suite?.project?.name}
          </Text>
        </div>
      ),
    },
    {
      title: '环境',
      key: 'environment',
      width: 100,
      render: (_, record) => (
        record.test_case?.environment?.name || record.test_suite?.environment?.name
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => (
        <Space>
          {getStatusIcon(status)}
          <Tag color={getStatusColor(status)}>
            {getStatusText(status)}
          </Tag>
        </Space>
      ),
    },
    {
      title: '成功/总数',
      key: 'test_results',
      width: 100,
      render: (_, record) => (
        <div>
          <Text>{record.passed_count}/{record.total_count}</Text>
          <Progress
            percent={record.total_count > 0 ? Math.round((record.passed_count / record.total_count) * 100) : 0}
            size="small"
            showInfo={false}
          />
        </div>
      ),
    },
    {
      title: '执行时长',
      dataIndex: 'duration',
      key: 'duration',
      width: 100,
      render: (duration: number) => `${Math.round(duration / 1000)}s`,
    },
    {
      title: '开始时间',
      dataIndex: 'start_time',
      key: 'start_time',
      width: 150,
      render: (date: string) => new Date(date).toLocaleString(),
    },
    {
      title: '操作',
      key: 'action',
      width: 150,
      render: (_, record) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => handleViewDetails(record)}
          >
            详情
          </Button>
          <Button
            type="link"
            size="small"
            icon={<DownloadOutlined />}
            onClick={() => handleDownloadReport(record)}
            disabled={record.status !== 'success' && record.status !== 'failed'}
          >
            报告
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={2}>测试报告</Title>
      
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="总执行次数" value={pagination.total} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="成功率"
              value={calculateSuccessRate(executions)}
              suffix="%"
              valueStyle={{ color: calculateSuccessRate(executions) >= 80 ? '#3f8600' : '#cf1322' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="运行中"
              value={executions.filter(e => e.status === 'running').length}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="失败次数"
              value={executions.filter(e => e.status === 'failed').length}
              valueStyle={{ color: '#cf1322' }}
            />
          </Card>
        </Col>
      </Row>

      <Card>
        <div style={{ marginBottom: 16 }}>
          <Space wrap>
            <Select
              placeholder="选择项目"
              style={{ width: 200 }}
              allowClear
              value={filters.project_id}
              onChange={(value) => setFilters({ ...filters, project_id: value })}
            >
              {projects.map(project => (
                <Option key={project.id} value={project.id}>
                  {project.name}
                </Option>
              ))}
            </Select>
            
            <Select
              placeholder="选择环境"
              style={{ width: 150 }}
              allowClear
              value={filters.environment_id}
              onChange={(value) => setFilters({ ...filters, environment_id: value })}
            >
              {environments.map(env => (
                <Option key={env.id} value={env.id}>
                  {env.name}
                </Option>
              ))}
            </Select>
            
            <Select
              placeholder="选择状态"
              style={{ width: 120 }}
              allowClear
              value={filters.status}
              onChange={(value) => setFilters({ ...filters, status: value })}
            >
              <Option value="success">成功</Option>
              <Option value="failed">失败</Option>
              <Option value="running">运行中</Option>
              <Option value="pending">等待中</Option>
            </Select>
            
            <RangePicker
              value={filters.date_range}
              onChange={(dates) => setFilters({ ...filters, date_range: dates })}
              placeholder={['开始日期', '结束日期']}
            />
            
            <Button icon={<ReloadOutlined />} onClick={loadExecutions}>
              刷新
            </Button>
          </Space>
        </div>

        <Table
          columns={columns}
          dataSource={executions}
          rowKey="id"
          loading={loading}
          pagination={{
            ...pagination,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total, range) =>
              `第 ${range[0]}-${range[1]} 条，共 ${total} 条`,
            onChange: (page, pageSize) => {
              setPagination({ ...pagination, current: page, pageSize: pageSize || 10 });
            },
          }}
        />
      </Card>

      <Drawer
        title="执行详情"
        placement="right"
        size="large"
        onClose={() => setIsDetailDrawerVisible(false)}
        open={isDetailDrawerVisible}
      >
        {selectedExecution && (
          <div>
            <Descriptions title="基本信息" bordered column={1} size="small">
              <Descriptions.Item label="执行ID">
                {selectedExecution.id}
              </Descriptions.Item>
              <Descriptions.Item label="执行类型">
                <Tag color={selectedExecution.execution_type === 'test_case' ? 'blue' : 'green'}>
                  {selectedExecution.execution_type === 'test_case' ? '测试用例' : '测试套件'}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="名称">
                {selectedExecution.test_case?.name || selectedExecution.test_suite?.name}
              </Descriptions.Item>
              <Descriptions.Item label="所属项目">
                {selectedExecution.test_case?.project?.name || selectedExecution.test_suite?.project?.name}
              </Descriptions.Item>
              <Descriptions.Item label="测试环境">
                {selectedExecution.test_case?.environment?.name || selectedExecution.test_suite?.environment?.name}
              </Descriptions.Item>
              <Descriptions.Item label="执行状态">
                <Space>
                  {getStatusIcon(selectedExecution.status)}
                  <Tag color={getStatusColor(selectedExecution.status)}>
                    {getStatusText(selectedExecution.status)}
                  </Tag>
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label="测试结果">
                <Space>
                  <Text>成功: {selectedExecution.passed_count}</Text>
                  <Text>失败: {selectedExecution.failed_count}</Text>
                  <Text>总计: {selectedExecution.total_count}</Text>
                </Space>
                <Progress
                  percent={selectedExecution.total_count > 0 ? 
                    Math.round((selectedExecution.passed_count / selectedExecution.total_count) * 100) : 0}
                  style={{ marginTop: 8 }}
                />
              </Descriptions.Item>
              <Descriptions.Item label="执行时长">
                {Math.round(selectedExecution.duration / 1000)} 秒
              </Descriptions.Item>
              <Descriptions.Item label="开始时间">
                {new Date(selectedExecution.start_time).toLocaleString()}
              </Descriptions.Item>
              <Descriptions.Item label="结束时间">
                {selectedExecution.end_time ? new Date(selectedExecution.end_time).toLocaleString() : '未结束'}
              </Descriptions.Item>
              {selectedExecution.error_message && (
                <Descriptions.Item label="错误信息">
                  <Text type="danger">{selectedExecution.error_message}</Text>
                </Descriptions.Item>
              )}
            </Descriptions>

            {executionLogs.length > 0 && (
              <div style={{ marginTop: 24 }}>
                <Title level={4}>执行日志</Title>
                <Timeline>
                  {executionLogs.map((log: any, index: number) => (
                    <Timeline.Item
                      key={index}
                      color={log.level === 'error' ? 'red' : log.level === 'warn' ? 'orange' : 'blue'}
                    >
                      <div>
                        <Badge
                          color={log.level === 'error' ? 'red' : log.level === 'warn' ? 'orange' : 'blue'}
                          text={log.level.toUpperCase()}
                        />
                        <Text style={{ marginLeft: 8, fontSize: '12px', color: '#999' }}>
                          {new Date(log.timestamp).toLocaleTimeString()}
                        </Text>
                      </div>
                      <div style={{ marginTop: 4 }}>
                        <Text>{log.message}</Text>
                      </div>
                      {log.details && (
                        <div style={{ marginTop: 4, padding: 8, background: '#f5f5f5', borderRadius: 4 }}>
                          <Text code style={{ fontSize: '12px' }}>{JSON.stringify(log.details, null, 2)}</Text>
                        </div>
                      )}
                    </Timeline.Item>
                  ))}
                </Timeline>
              </div>
            )}

            {executionScreenshots.length > 0 && (
              <div style={{ marginTop: 24 }}>
                <Title level={4}>截图记录</Title>
                <List
                  grid={{ gutter: 16, column: 2 }}
                  dataSource={executionScreenshots}
                  renderItem={(screenshot: any, index: number) => (
                    <List.Item>
                      <Card
                        hoverable
                        cover={
                          <Image
                            alt={`screenshot-${index}`}
                            src={screenshot.url}
                            preview={{
                              mask: <EyeOutlined />,
                            }}
                            style={{ height: 150, objectFit: 'cover' }}
                          />
                        }
                      >
                        <Card.Meta
                          title={`步骤 ${screenshot.step_index + 1}`}
                          description={
                            <div>
                              <div>{screenshot.description}</div>
                              <Text type="secondary" style={{ fontSize: '12px' }}>
                                {new Date(screenshot.timestamp).toLocaleString()}
                              </Text>
                            </div>
                          }
                        />
                      </Card>
                    </List.Item>
                  )}
                />
              </div>
            )}

            {executionLogs.length === 0 && executionScreenshots.length === 0 && (
              <Empty 
                description="暂无详细信息" 
                style={{ marginTop: 40 }}
              />
            )}
          </div>
        )}
      </Drawer>
    </div>
  );
};

export default Reports;